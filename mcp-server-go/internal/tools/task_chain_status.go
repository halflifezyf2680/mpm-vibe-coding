package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func renderTaskChainStatus(sm *SessionManager, taskID string) (*mcp.CallToolResult, error) {
	if strings.TrimSpace(taskID) == "" {
		return mcp.NewToolResultError("status 模式需要 task_id 参数"), nil
	}

	if sm.TaskChainsV2 != nil {
		if chain, ok := sm.TaskChainsV2[taskID]; ok {
			steps := append([]Step(nil), chain.Steps...)
			sort.Slice(steps, func(i, j int) bool {
				if stepNumberEqual(steps[i].Number, steps[j].Number) {
					return steps[i].Name < steps[j].Name
				}
				return steps[i].Number < steps[j].Number
			})

			type stepJSON struct {
				Number  string `json:"number"`
				Status  string `json:"status"`
				Name    string `json:"name"`
				Input   string `json:"input,omitempty"`
				Summary string `json:"summary,omitempty"`
			}

			var stepsOut []stepJSON
			for _, s := range steps {
				stepsOut = append(stepsOut, stepJSON{
					Number:  formatStepNumber(s.Number),
					Status:  string(s.Status),
					Name:    s.Name,
					Input:   s.Input,
					Summary: s.Summary,
				})
			}

			out := map[string]interface{}{
				"task_id":      taskID,
				"version":      "v2",
				"status":       string(chain.Status),
				"current_step": formatStepNumber(chain.CurrentStep),
				"description":  chain.Description,
				"steps":        stepsOut,
			}

			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("序列化失败: %v", err)), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		}
	}

	if sm.TaskChains != nil {
		if chain, ok := sm.TaskChains[taskID]; ok {
			type stepJSON struct {
				Number int    `json:"number"`
				Status string `json:"status"`
				Name   string `json:"name"`
			}
			var stepsOut []stepJSON
			for i, p := range chain.Plan {
				status := "todo"
				if i < chain.CurrentStep {
					status = "done"
				} else if i == chain.CurrentStep {
					status = "current"
				}
				stepsOut = append(stepsOut, stepJSON{Number: i + 1, Status: status, Name: p})
			}
			out := map[string]interface{}{
				"task_id":      taskID,
				"version":      "v1",
				"status":       chain.Status,
				"current_step": chain.CurrentStep,
				"total_steps":  len(chain.Plan),
				"steps":        stepsOut,
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("序列化失败: %v", err)), nil
			}
			return mcp.NewToolResultText(string(data)), nil
		}
	}

	return mcp.NewToolResultError(fmt.Sprintf("任务 %s 不存在", taskID)), nil
}
