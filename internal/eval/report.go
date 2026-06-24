package eval

import (
	"bytes"
	"fmt"
	"html/template"
)

const htmlTmpl = `<!doctype html>
<html><head><meta charset="utf-8"><title>{{.SkillName}} — Skill Forge eval</title>
<style>
body{font-family:-apple-system,Segoe UI,Roboto,sans-serif;background:#15151f;color:#e6edf3;margin:0;padding:2rem;}
h1{color:#ff8c42;margin:0;}
.meta{color:#8b8b9a;}
.summary{display:flex;gap:1.5rem;margin:1.5rem 0;}
.metric{background:#1e1e2a;border:1px solid #2a2a3a;border-radius:10px;padding:.8rem 1.4rem;}
.metric .v{font-size:1.8rem;font-weight:700;color:#3fb950;}
.case{background:#1e1e2a;border:1px solid #2a2a3a;border-radius:10px;padding:1rem 1.25rem;margin:1rem 0;}
.case h3{margin:0 0 .3rem;color:#36d7b7;}
.exp{margin:.15rem 0;}
.pass{color:#3fb950;} .fail{color:#f85149;}
pre{white-space:pre-wrap;background:#12121a;padding:.7rem;border-radius:8px;color:#c9d1d9;max-height:280px;overflow:auto;}
.cols{display:grid;grid-template-columns:1fr 1fr;gap:1rem;}
</style></head><body>
<h1>◆ {{.SkillName}}</h1>
<div class="meta">{{.Provider}} · {{.Model}}</div>
<div class="summary">
  <div class="metric"><div>with skill</div><div class="v">{{pct .WithSkillPassRate}}</div></div>
  {{if .Baseline}}<div class="metric"><div>baseline</div><div class="v" style="color:#8b8b9a">{{pct .BaselinePassRate}}</div></div>{{end}}
</div>
{{range .Cases}}<div class="case">
  <h3>Eval {{.ID}}</h3>
  <div class="meta">{{.Prompt}}</div>
  <div class="cols">
    <div><strong>with skill — {{pct .WithSkill.PassRate}}</strong>
      {{range .WithSkill.Expectations}}<div class="exp {{if .Passed}}pass{{else}}fail{{end}}">{{if .Passed}}✓{{else}}✗{{end}} {{.Text}}</div>{{end}}
      <pre>{{.WithSkill.Output}}</pre></div>
    {{if .Baseline}}<div><strong>baseline — {{pct .Baseline.PassRate}}</strong>
      {{range .Baseline.Expectations}}<div class="exp {{if .Passed}}pass{{else}}fail{{end}}">{{if .Passed}}✓{{else}}✗{{end}} {{.Text}}</div>{{end}}
      <pre>{{.Baseline.Output}}</pre></div>{{end}}
  </div>
</div>{{end}}
</body></html>`

// HTML renders the report as a standalone HTML page.
func (r *Report) HTML() (string, error) {
	t, err := template.New("report").Funcs(template.FuncMap{
		"pct": func(f float64) string { return fmt.Sprintf("%.0f%%", f*100) },
	}).Parse(htmlTmpl)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := t.Execute(&b, r); err != nil {
		return "", err
	}
	return b.String(), nil
}
