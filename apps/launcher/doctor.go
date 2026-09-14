package main

import "strings"

// formatDoctor turns a doctor response into a short, actionable summary:
// "全部就绪" when every check passes, else the failed messages + actions.
func formatDoctor(res map[string]any) string {
	checks, _ := res["checks"].([]any)
	failed := make([]string, 0, len(checks))
	for _, ch := range checks {
		c, ok := ch.(map[string]any)
		if !ok {
			continue
		}
		if okFlag, _ := c["ok"].(bool); okFlag {
			continue
		}
		msg, _ := c["message"].(string)
		action, _ := c["action"].(string)
		line := msg
		if strings.TrimSpace(action) != "" {
			line += " → " + action
		}
		failed = append(failed, line)
	}
	if len(failed) == 0 {
		return "全部就绪"
	}
	return strings.Join(failed, "；")
}
