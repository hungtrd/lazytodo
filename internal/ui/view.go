package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/task"
)

func (m Model) View() string {
	// Layout
	totalWidth := max(30, m.width)
	sections := make([]string, 0, len(statusOrder))
	frameW, _ := columnStyle.GetFrameSize()
	gapW := 0
	if !m.vertical {
		gapW = 1
	}

	if m.vertical {
		contentW := max(1, totalWidth-frameW)
		for _, st := range statusOrder {
			title := statusTitle(st)
			items := m.renderItems(st)
			header := headerStyle.Render(fmt.Sprintf("%s (%d)", title, len(m.tasksByStatus[st])))
			content := header + "\n" + strings.Join(items, "\n")
			style := unfocusedColStyle
			if m.focused == st {
				style = focusedColStyle
			}
			sections = append(sections, style.Width(contentW).Render(content))
		}
	} else {
		numCols := len(statusOrder)
		contentTotal := totalWidth - numCols*frameW - (numCols-1)*gapW
		if contentTotal < numCols {
			contentTotal = numCols
		}
		base := contentTotal / numCols
		rem := contentTotal - base*numCols
		widths := make([]int, numCols)
		contents := make([]string, numCols)
		styles := make([]lipgloss.Style, numCols)
		for i := 0; i < numCols; i++ {
			widths[i] = base
			if i < rem {
				widths[i]++
			}
		}
		for i, st := range statusOrder {
			title := statusTitle(st)
			items := m.renderItems(st)
			header := headerStyle.Render(fmt.Sprintf("%s (%d)", title, len(m.tasksByStatus[st])))
			contents[i] = header + "\n" + strings.Join(items, "\n")
			style := unfocusedColStyle
			if m.focused == st {
				style = focusedColStyle
			}
			styles[i] = style
			sections = append(sections, style.Width(max(1, widths[i])).Render(contents[i]))
		}
		gap := lipgloss.NewStyle().Width(gapW).Render(" ")
		boardCandidate := lipgloss.JoinHorizontal(lipgloss.Top, interleave(sections, gap)...)
		diff := totalWidth - lipgloss.Width(boardCandidate)
		if diff != 0 {
			last := numCols - 1
			newW := max(1, widths[last]+diff)
			sections[last] = styles[last].Width(newW).Render(contents[last])
		}
	}
	var board string
	if m.vertical {
		board = lipgloss.JoinVertical(lipgloss.Left, sections...)
	} else {
		gap := lipgloss.NewStyle().Width(gapW).Render(" ")
		board = lipgloss.JoinHorizontal(lipgloss.Top, interleave(sections, gap)...)
	}

	help := m.renderHelp(totalWidth)

	if m.mode == modeNew {
		prompt := footerStyle.Copy().Bold(true).Render("New Task:")
		return board + "\n" + prompt + "\n" + m.input.View() + "\n" + help
	}
	if m.mode == modeEdit {
		prompt := footerStyle.Copy().Bold(true).Render("Edit Task:")
		return board + "\n" + prompt + "\n" + m.input.View() + "\n" + help
	}
	return board + "\n" + help
}

func (m Model) renderItems(status domain.TaskStatus) []string {
	list := append([]domain.Task(nil), m.tasksByStatus[status]...)
	order := task.SortedOrder(list)

	indexInOriginal := func(tk domain.Task) int {
		for i, t := range m.tasksByStatus[status] {
			if t.Id == tk.Id {
				return i
			}
		}
		return -1
	}

	lines := make([]string, 0, len(order))
	for _, ordIdx := range order {
		t := list[ordIdx]
		star := "  "
		if t.IsStarred {
			star = starredStyle.Render("★ ")
		}
		baseText := t.Content
		isSelected := indexInOriginal(t) == m.selectedIdx[status] && m.focused == status && m.mode == modeList

		var textStyled string
		if isSelected {
			style := selectedTextStyle
			if status == domain.TaskStatusDone {
				style = style.Copy().Strikethrough(true)
			}
			textStyled = style.Render(baseText)
		} else {
			if status == domain.TaskStatusDone {
				textStyled = doneStyle.Render(baseText)
			} else {
				textStyled = baseText
			}
		}

		var left string
		if isSelected {
			left = cursorBullet + " "
		} else {
			left = "  "
		}
		line := left + star + textStyled
		lines = append(lines, line)
	}
	return lines
}

// renderHelp lays out the shortcut hints across multiple columns
func (m Model) renderHelp(totalWidth int) string {
	items := []string{
		"h/l: focus column",
		"j/k: move",
		"g/G: top/bottom",
		"[ \\ / ]: move task",
		"space/x: toggle done",
		"s: star",
		"n: new",
		"e: edit",
		"d/backspace/del: delete",
		"v: toggle layout",
		"q: quit",
		"esc: cancel",
	}

	minColWidth := 22
	gapW := 2
	maxCols := 3
	if m.vertical {
		maxCols = 2
	}
	cols := totalWidth / (minColWidth + gapW)
	if cols < 1 {
		cols = 1
	}
	if cols > maxCols {
		cols = maxCols
	}

	rows := (len(items) + cols - 1) / cols
	columns := make([]string, 0, cols)
	contentTotal := totalWidth - (cols-1)*gapW
	base := contentTotal / cols
	rem := contentTotal - base*cols
	for c := 0; c < cols; c++ {
		lines := make([]string, 0, rows)
		for r := 0; r < rows; r++ {
			idx := r + c*rows
			if idx < len(items) {
				lines = append(lines, items[idx])
			} else {
				lines = append(lines, "")
			}
		}
		w := base
		if c < rem {
			w++
		}
		col := footerStyle.Width(max(1, w)).Render(strings.Join(lines, "\n"))
		columns = append(columns, col)
	}
	gap := lipgloss.NewStyle().Width(gapW).Render(" ")
	return lipgloss.JoinHorizontal(lipgloss.Top, interleave(columns, gap)...)
}
