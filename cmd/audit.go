package cmd

import (
	"fmt"

	"github.com/mrdulasolutions/skillforge/internal/compliance"
	"github.com/mrdulasolutions/skillforge/internal/tui"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Inspect a skill's compliance audit log",
	Long:  "Work with the HMAC-chained, append-only audit log written under <skill>/.skillforge when the compliance profile is enabled.",
}

var auditVerifyCmd = &cobra.Command{
	Use:   "verify [path]",
	Short: "Verify a skill's HMAC-chained audit log",
	Long:  "Recompute the audit log's HMAC chain and report whether it is intact. Exits non-zero if the chain is broken (tamper-evident).",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAuditVerify,
}

func init() {
	auditCmd.AddCommand(auditVerifyCmd)
}

func runAuditVerify(_ *cobra.Command, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	header("audit verify")

	if !compliance.HasLog(path) {
		return fmt.Errorf("no audit log at %s — this skill has no compliance profile", path)
	}
	v, err := compliance.Verify(path)
	if err != nil {
		return err
	}
	if v.OK {
		fmt.Println(tui.OK(fmt.Sprintf("audit chain intact — %d entries verified", v.Lines)))
		return nil
	}
	fmt.Println(tui.Err(fmt.Sprintf("audit chain BROKEN at entry %d: %s", v.BrokenAt, v.Reason)))
	return fmt.Errorf("audit verification failed")
}
