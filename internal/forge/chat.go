package forge

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrdulasolutions/skillforge/internal/ai"
	"github.com/mrdulasolutions/skillforge/internal/tui"
)

type role int

const (
	roleUser role = iota
	roleAssistant
	roleSystem
)

type phase int

const (
	phaseInterview phase = iota
	phaseReady
	phaseReview
)

type chatMsg struct {
	role role
	text string
}

// deltaEv is the internal producer→reader event (not a tea.Msg).
type deltaEv struct {
	s    string
	done bool
	err  error
}

type submitMsg struct{ text string }
type streamDeltaMsg struct{ delta string }
type streamDoneMsg struct {
	full string
	err  error
}
type draftDoneMsg struct {
	spec *ai.SkillSpec
	err  error
}

const (
	headerH = 2
	footerH = 1
)

const anvilArt = ` ┌─────┐
 └──┬──┘
┌───┴───┐
└───────┘`

var (
	chatInputBox = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)

	userLabel  = lipgloss.NewStyle().Foreground(tui.ColPrimary).Bold(true)
	userBubble = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), false, true, false, false).
			BorderForeground(tui.ColPrimary).
			Foreground(tui.ColText).
			PaddingRight(1)
	asstLabel = lipgloss.NewStyle().Foreground(tui.ColAccent).Bold(true)

	phaseChipStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1A1206")).
			Background(tui.ColPrimary).
			Bold(true).
			Padding(0, 1)
)

type model struct {
	ctx    context.Context
	p      ai.Provider
	draft  Drafter
	seed   tui.WizardResult
	parent string

	vp viewport.Model
	ta textarea.Model
	sp spinner.Model

	msgs       []chatMsg
	transcript []ai.Message
	phase      phase
	spec       *ai.SkillSpec

	busy     bool
	streamCh chan deltaEv
	pending  strings.Builder

	width, height int
	follow        bool
	ready         bool

	result  *tui.WizardResult
	degrade bool
}

func newModel(ctx context.Context, p ai.Provider, draft Drafter, seed tui.WizardResult, parent string) model {
	ta := textarea.New()
	ta.Placeholder = "Describe the skill you want — plain words…"
	ta.Prompt = tui.Subtitle.Render("› ")
	ta.CharLimit = 4000
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.SetHeight(1)
	ta.Focus()

	sp := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(tui.ColPrimary)),
	)

	return model{
		ctx: ctx, p: p, draft: draft, seed: seed, parent: parent,
		ta: ta, sp: sp, phase: phaseInterview, follow: true,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink}
	if s := strings.TrimSpace(m.seed.Description); s != "" {
		cmds = append(cmds, func() tea.Msg { return submitMsg{text: s} })
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.relayout()
		m.ready = true
		m.refreshViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			if !m.busy && strings.TrimSpace(m.ta.Value()) != "" {
				m.ta.Reset()
				m.growInput()
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyCtrlJ:
			if !m.busy {
				m.ta.InsertRune('\n')
				m.growInput()
			}
			return m, nil
		case tea.KeyEnter:
			if m.busy {
				return m, nil
			}
			line := strings.TrimSpace(m.ta.Value())
			if line == "" {
				return m, nil
			}
			m.ta.Reset()
			m.growInput()
			return m, func() tea.Msg { return submitMsg{text: line} }
		}
		if !m.busy {
			m.ta, cmd = m.ta.Update(msg)
			cmds = append(cmds, cmd)
			m.growInput()
		}
		m.vp, cmd = m.vp.Update(msg)
		cmds = append(cmds, cmd)
		m.follow = m.vp.AtBottom()
		return m, tea.Batch(cmds...)

	case submitMsg:
		return m.handleSubmit(msg.text)

	case streamDeltaMsg:
		m.pending.WriteString(msg.delta)
		m.refreshViewport()
		return m, waitForDelta(m.streamCh)

	case streamDoneMsg:
		m.busy = false
		m.streamCh = nil
		if msg.err != nil {
			return m.failOrCancel(msg.err)
		}
		reply := msg.full
		if strings.TrimSpace(reply) == "" {
			reply = m.pending.String()
		}
		m.pending.Reset()
		m.msgs = append(m.msgs, chatMsg{roleAssistant, reply})
		m.transcript = append(m.transcript, ai.Message{Role: "assistant", Content: reply})
		if strings.Contains(reply, "?") {
			m.phase = phaseInterview
		} else {
			m.phase = phaseReady
			m.msgs = append(m.msgs, chatMsg{roleSystem, `say "go" to build it, or add more detail`})
		}
		m.syncPlaceholder()
		m.ta.Focus()
		m.refreshViewport()
		return m, textarea.Blink

	case draftDoneMsg:
		m.busy = false
		if msg.err != nil {
			return m.failOrCancel(msg.err)
		}
		m.spec = repair(msg.spec, m.parent)
		m.phase = phaseReview
		m.msgs = append(m.msgs, chatMsg{roleAssistant, cardString(m.spec)})
		m.syncPlaceholder()
		m.ta.Focus()
		m.refreshViewport()
		return m, textarea.Blink

	case spinner.TickMsg:
		if m.busy {
			m.sp, cmd = m.sp.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	if !m.busy {
		m.ta, cmd = m.ta.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) handleSubmit(line string) (tea.Model, tea.Cmd) {
	if isCancel(line) {
		return m, tea.Quit
	}
	m.msgs = append(m.msgs, chatMsg{roleUser, line})
	m.follow = true
	m.transcript = append(m.transcript, ai.Message{Role: "user", Content: line})

	switch {
	case m.phase == phaseReview:
		if isAffirmative(line) || isCreateCmd(line) {
			res := finalize(m.spec, m.seed)
			m.result = &res
			return m, tea.Quit
		}
		return m.startDraft(m.spec, line)
	case isCreateCmd(line) || (m.phase == phaseReady && isAffirmative(line)):
		return m.startDraft(nil, "")
	default:
		return m.startInterview()
	}
}

func (m model) failOrCancel(err error) (tea.Model, tea.Cmd) {
	if m.ctx.Err() != nil {
		return m, tea.Quit
	}
	if c := firstUserConcept(m.transcript, m.seed.Description); c != "" {
		m.seed.Description = c
	}
	m.degrade = true
	_ = err // surfaced to the caller as ErrDegrade
	return m, tea.Quit
}

func firstUserConcept(t []ai.Message, fallback string) string {
	for _, msg := range t {
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			return msg.Content
		}
	}
	return fallback
}

// --- async commands ---

func (m model) startInterview() (tea.Model, tea.Cmd) {
	m.busy = true
	m.pending.Reset()
	m.ta.Blur()
	ch := make(chan deltaEv, 64)
	m.streamCh = ch
	req := ai.Request{
		Model:       ai.DefaultModel(m.p),
		System:      ai.InterviewSystem,
		Messages:    m.transcript,
		Temperature: 0.5,
		MaxTokens:   500,
	}
	return m, tea.Batch(m.sp.Tick, streamCmd(m.ctx, m.p, req, ch), waitForDelta(ch))
}

func streamCmd(ctx context.Context, p ai.Provider, req ai.Request, ch chan deltaEv) tea.Cmd {
	return func() tea.Msg {
		s, ok := p.(ai.Streamer)
		if !ok {
			resp, err := p.Complete(ctx, req)
			if err != nil {
				ch <- deltaEv{done: true, err: err}
			} else {
				ch <- deltaEv{s: resp.Text}
				ch <- deltaEv{done: true, s: resp.Text}
			}
			return nil
		}
		resp, err := s.Stream(ctx, req, func(d string) {
			select {
			case ch <- deltaEv{s: d}:
			case <-ctx.Done():
			}
		})
		if err != nil {
			ch <- deltaEv{done: true, err: err}
			return nil
		}
		ch <- deltaEv{done: true, s: resp.Text}
		return nil
	}
}

func waitForDelta(ch chan deltaEv) tea.Cmd {
	return func() tea.Msg {
		ev, open := <-ch
		if !open {
			return streamDoneMsg{}
		}
		if ev.done {
			return streamDoneMsg{full: ev.s, err: ev.err}
		}
		return streamDeltaMsg{delta: ev.s}
	}
}

func (m model) startDraft(prior *ai.SkillSpec, instruction string) (tea.Model, tea.Cmd) {
	m.busy = true
	m.ta.Blur()
	m.msgs = append(m.msgs, chatMsg{roleSystem, tui.GlyphSpark + " drafting your skill…"})
	m.refreshViewport()
	return m, tea.Batch(m.sp.Tick, draftCmd(m.ctx, m.draft, m.transcript, prior, instruction))
}

func draftCmd(ctx context.Context, draft Drafter, transcript []ai.Message, prior *ai.SkillSpec, instr string) tea.Cmd {
	return func() tea.Msg {
		spec, err := draft(ctx, transcript, prior, instr)
		return draftDoneMsg{spec: spec, err: err}
	}
}

// --- layout & rendering ---

func (m *model) inputZoneH() int { return m.ta.Height() + 2 }

func (m *model) relayout() {
	m.ta.SetWidth(m.width - 4)
	vpH := m.height - headerH - footerH - m.inputZoneH()
	if vpH < 1 {
		vpH = 1
	}
	if m.vp.Width == 0 {
		m.vp = viewport.New(m.width, vpH)
	} else {
		m.vp.Width, m.vp.Height = m.width, vpH
	}
}

// growInput sizes the input box (1–4 rows) to the typed content.
func (m *model) growInput() {
	n := strings.Count(m.ta.Value(), "\n") + 1
	if n < 1 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	if m.ta.Height() != n {
		m.ta.SetHeight(n)
		if m.ready {
			m.relayout()
			m.refreshViewport()
		}
	}
}

func (m *model) syncPlaceholder() {
	switch m.phase {
	case phaseReview:
		m.ta.Placeholder = `say "go" to build, or tell me what to change…`
	case phaseReady:
		m.ta.Placeholder = `say "go" to build, or add more detail…`
	default:
		m.ta.Placeholder = "Describe the skill you want — plain words…"
	}
}

func (m *model) refreshViewport() {
	m.vp.SetContent(m.renderMessages())
	if m.follow {
		m.vp.GotoBottom()
	}
}

func (m model) renderMessages() string {
	w := m.vp.Width
	if w < 1 {
		w = 1
	}
	var b strings.Builder
	b.WriteString(m.welcomeView(w))
	b.WriteString("\n\n")
	for _, c := range m.msgs {
		switch c.role {
		case roleUser:
			block := lipgloss.JoinVertical(lipgloss.Right,
				userLabel.Render("you ›"), userBubble.Render(c.text))
			b.WriteString(lipgloss.PlaceHorizontal(w, lipgloss.Right, block))
		case roleAssistant:
			b.WriteString(asstLabel.Render("forge ◆") + "\n" + tui.RenderMarkdown(c.text))
		case roleSystem:
			b.WriteString(tui.Muted.Render(tui.GlyphSpark + " " + c.text))
		}
		b.WriteString("\n\n")
	}
	if m.busy && m.pending.Len() > 0 {
		b.WriteString(asstLabel.Render("forge ◆") + "\n" +
			m.pending.String() + lipgloss.NewStyle().Foreground(tui.ColPrimary).Render("▏"))
	}
	return lipgloss.NewStyle().Width(w).Render(b.String())
}

func (m model) welcomeView(width int) string {
	if width > 78 {
		width = 78
	}
	innerW := width - 4
	mascot := lipgloss.NewStyle().Foreground(tui.ColPrimary).Render(anvilArt)
	steps := lipgloss.JoinVertical(lipgloss.Left,
		tui.Subtitle.Render("How this works"),
		tui.Val.Render("1. Describe a skill in plain words"),
		tui.Val.Render("2. I draft it — refine by chatting"),
		tui.Val.Render(`3. Say "go" and I write the files`),
	)
	var top string
	if innerW >= 50 {
		top = lipgloss.JoinHorizontal(lipgloss.Top, mascot, "     ", steps)
	} else {
		top = lipgloss.JoinVertical(lipgloss.Left, mascot, "", steps)
	}
	modelLine := tui.Muted.Render(truncateStr(m.p.Name()+" · "+ai.DefaultModel(m.p), innerW))
	body := lipgloss.JoinVertical(lipgloss.Left, top, "", modelLine)
	return tui.TitledBox("Skill Forge", body, width)
}

func (m model) headerView() string {
	left := tui.CompactBanner()
	chip := phaseChipStyle.Render(m.phaseWord())
	full := ai.DefaultModel(m.p)
	right := chip + "  " + tui.Muted.Render(shortModel(full))
	if cwd := shortCwd(); cwd != "" {
		cand := chip + "  " + tui.Muted.Render(cwd+" · "+shortModel(full))
		if lipgloss.Width(left)+lipgloss.Width(cand)+2 <= m.width {
			right = cand
		}
	}
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	bar := left + strings.Repeat(" ", pad) + right
	return bar + "\n" + tui.GradientRule(m.width)
}

func (m model) footerView() string {
	var keys []string
	if m.phase == phaseReview {
		keys = []string{"enter send", `"go" to build`, "type to refine", "esc cancel"}
	} else {
		keys = []string{"enter send", "ctrl+j newline", "↑↓ scroll", "esc cancel"}
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = tui.Muted.Render(k)
	}
	return lipgloss.NewStyle().Width(m.width).Render(strings.Join(parts, tui.Muted.Render(" · ")))
}

func (m model) phaseWord() string {
	if m.busy {
		return "working"
	}
	switch m.phase {
	case phaseReady:
		return "ready"
	case phaseReview:
		return "review"
	default:
		return "interview"
	}
}

func (m model) View() string {
	if !m.ready {
		return ""
	}
	if m.height < 8 || m.width < 40 {
		return "terminal too small — resize to continue"
	}
	var input string
	if m.busy {
		input = chatInputBox.BorderForeground(tui.ColMuted).
			Render(m.sp.View() + " " + tui.Muted.Render("working…"))
	} else {
		input = chatInputBox.BorderForeground(tui.ColPrimary).Render(m.ta.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerView(), m.vp.View(), input, m.footerView())
}

// --- small helpers ---

func shortModel(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return "—"
	}
	return truncateStr(s, 24)
}

func shortCwd() string {
	d, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Base(d)
}

func truncateStr(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

// Chat runs the full-screen conversational TUI starting from the interview and
// returns the result for cmd/new.go: (result, ok, err). ok=false with a nil
// error means the user cancelled; ErrDegrade means fall back to the offline form.
func Chat(ctx context.Context, p ai.Provider, draft Drafter, seed tui.WizardResult, parent string) (tui.WizardResult, bool, error) {
	return runChat(ctx, newModel(ctx, p, draft, seed, parent), seed)
}

// ChatFromDraft opens the chat already in the review phase, showing a skill that
// was drafted elsewhere (e.g. by `compile`) for the user to refine and confirm.
func ChatFromDraft(ctx context.Context, p ai.Provider, draft Drafter, seed tui.WizardResult, parent string, spec *ai.SkillSpec) (tui.WizardResult, bool, error) {
	m := newModel(ctx, p, draft, seed, parent)
	m.spec = repair(spec, parent)
	m.phase = phaseReview
	m.transcript = []ai.Message{{Role: "user", Content: "I provided reference material; you drafted this skill."}}
	m.msgs = append(m.msgs,
		chatMsg{roleSystem, "Drafted a skill from your material — review and refine below."},
		chatMsg{roleAssistant, cardString(m.spec)},
	)
	m.syncPlaceholder()
	return runChat(ctx, m, seed)
}

func runChat(ctx context.Context, m model, seed tui.WizardResult) (tui.WizardResult, bool, error) {
	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	final, err := prog.Run()
	if err != nil {
		if ctx.Err() != nil {
			return seed, false, nil
		}
		return seed, false, err
	}
	fm, ok := final.(model)
	if !ok {
		return seed, false, nil
	}
	switch {
	case fm.degrade:
		return fm.seed, false, ErrDegrade
	case fm.result != nil:
		return *fm.result, true, nil
	default:
		return seed, false, nil
	}
}
