package main

import (
    "fmt"
    "os"
    "sort"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/lipgloss"

    "github.com/hungtrd/lazytodo/internal/domain"
    "github.com/hungtrd/lazytodo/internal/storage"
)

type uiMode int

const (
    modeList uiMode = iota
    modeNew
    modeEdit
)

var (
    statusOrder = []domain.TaskStatus{
        domain.TaskStatusTodo,
        domain.TaskStatusInProgress,
        domain.TaskStatusDone,
    }

    headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
    columnStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 1)
    focusedColStyle  = columnStyle.Copy().BorderForeground(lipgloss.Color("12"))
    unfocusedColStyle = columnStyle.Copy().BorderForeground(lipgloss.Color("240"))
    selectedItemStyle = lipgloss.NewStyle().Reverse(true).Bold(true)
    starredStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
    doneStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Strikethrough(true)
    footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1)
    cursorBullet      = "•"
)

type model struct {
    width  int
    height int

    tasksByStatus map[domain.TaskStatus][]domain.Task
    selectedIdx   map[domain.TaskStatus]int
    focused       domain.TaskStatus

    mode       uiMode
    input      textinput.Model
    editingRef *taskRef

    // layout orientation: false = horizontal (default), true = vertical
    vertical bool
}

type taskRef struct {
    status domain.TaskStatus
    index  int
}

func initialTasks() map[domain.TaskStatus][]domain.Task {
    // load from storage; if empty, start with an empty board
    m, err := storage.LoadTasks()
    if err != nil {
        // fallback to empty board on load error
        m = map[domain.TaskStatus][]domain.Task{
            domain.TaskStatusTodo:       {},
            domain.TaskStatusInProgress: {},
            domain.TaskStatusDone:       {},
        }
    }
    return m
}

func initialModel() model {
    ti := textinput.New()
    ti.Placeholder = "Task content..."
    ti.Prompt = "➤ "
    ti.CharLimit = 256
    return model{
        tasksByStatus: initialTasks(),
        selectedIdx: map[domain.TaskStatus]int{
            domain.TaskStatusTodo:       0,
            domain.TaskStatusInProgress: 0,
            domain.TaskStatusDone:       0,
        },
        focused: domain.TaskStatusTodo,
        mode:    modeList,
        input:   ti,
        vertical: false,
    }
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        return m, nil
    case tea.KeyMsg:
        if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
            return m, tea.Quit
        }
        switch m.mode {
        case modeList:
            return m.updateListMode(msg)
        case modeNew, modeEdit:
            return m.updateInputMode(msg)
        }
    }
    return m, nil
}

func (m model) updateListMode(key tea.KeyMsg) (tea.Model, tea.Cmd) {
    col := m.focused
    items := m.tasksByStatus[col]
    cur := m.selectedIdx[col]

    switch key.String() {
    case "up", "k":
        if len(items) == 0 { return m, nil }
        order := m.sortedOrder(col)
        pos := indexOf(order, cur)
        if pos == -1 { pos = 0 }
        if pos > 0 { pos-- }
        m.selectedIdx[col] = order[pos]
    case "down", "j":
        if len(items) == 0 { return m, nil }
        order := m.sortedOrder(col)
        pos := indexOf(order, cur)
        if pos == -1 { pos = 0 }
        if pos < len(order)-1 { pos++ }
        m.selectedIdx[col] = order[pos]
    case "left", "h":
        m.focused = prevStatus(m.focused)
    case "right", "l":
        m.focused = nextStatus(m.focused)
    case "[", "\\":
        m = m.moveTask(col, cur, prevStatus(col))
    case "]", "/":
        m = m.moveTask(col, cur, nextStatus(col))
    case " ", "x":
        // toggle done: if not done -> move to done; if done -> move to todo
        target := domain.TaskStatusDone
        if col == domain.TaskStatusDone { target = domain.TaskStatusTodo }
        m = m.moveTask(col, cur, target)
    case "s":
        if len(items) == 0 { return m, nil }
        it := items[cur]
        it.IsStarred = !it.IsStarred
        m.tasksByStatus[col][cur] = it
        _ = storage.SaveTasks(m.tasksByStatus)
    case "n":
        m.mode = modeNew
        m.input.SetValue("")
        m.input.Focus()
        return m, textinput.Blink
    case "e":
        if len(items) == 0 { return m, nil }
        m.mode = modeEdit
        m.editingRef = &taskRef{status: col, index: cur}
        m.input.SetValue(items[cur].Content)
        m.input.CursorEnd()
        m.input.Focus()
        return m, textinput.Blink
    case "backspace", "delete":
        if len(items) == 0 { return m, nil }
        m.deleteTask(col, cur)
        _ = storage.SaveTasks(m.tasksByStatus)
    case "g":
        if len(items) > 0 {
            order := m.sortedOrder(col)
            m.selectedIdx[col] = order[0]
        }
    case "G":
        if len(items) > 0 {
            order := m.sortedOrder(col)
            m.selectedIdx[col] = order[len(order)-1]
        }
    case "v":
        m.vertical = !m.vertical
    }
    return m, nil
}

func (m model) updateInputMode(key tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch key.Type {
    case tea.KeyEsc:
        m.mode = modeList
        m.editingRef = nil
        m.input.Blur()
        return m, nil
    case tea.KeyEnter:
        content := strings.TrimSpace(m.input.Value())
        if content != "" {
            if m.mode == modeNew {
                m.addTask(content)
            } else if m.mode == modeEdit && m.editingRef != nil {
                ref := *m.editingRef
                t := m.tasksByStatus[ref.status][ref.index]
                t.Content = content
                t.UpdatedAt = time.Now().Unix()
                m.tasksByStatus[ref.status][ref.index] = t
                m.editingRef = nil
                _ = storage.SaveTasks(m.tasksByStatus)
            }
        }
        m.mode = modeList
        m.input.Blur()
        return m, nil
    }
    var cmd tea.Cmd
    m.input, cmd = m.input.Update(key)
    return m, cmd
}

func (m *model) addTask(content string) {
    now := time.Now().Unix()
    t := domain.Task{Id: newID(), Content: content, Status: domain.TaskStatusTodo, CreatedAt: now}
    m.tasksByStatus[domain.TaskStatusTodo] = append([]domain.Task{t}, m.tasksByStatus[domain.TaskStatusTodo]...)
    m.focused = domain.TaskStatusTodo
    m.selectedIdx[domain.TaskStatusTodo] = 0
    _ = storage.SaveTasks(m.tasksByStatus)
}

func (m model) moveTask(from domain.TaskStatus, index int, to domain.TaskStatus) model {
    if from == to { return m }
    list := m.tasksByStatus[from]
    if index < 0 || index >= len(list) { return m }
    task := list[index]
    // remove from source
    m.tasksByStatus[from] = append(list[:index], list[index+1:]...)
    // insert at top of target
    task.Status = to
    task.UpdatedAt = time.Now().Unix()
    m.tasksByStatus[to] = append([]domain.Task{task}, m.tasksByStatus[to]...)
    // adjust selection
    if index >= len(m.tasksByStatus[from]) {
        m.selectedIdx[from] = max(0, len(m.tasksByStatus[from])-1)
    }
    m.focused = to
    m.selectedIdx[to] = 0
    _ = storage.SaveTasks(m.tasksByStatus)
    return m
}

func (m *model) deleteTask(status domain.TaskStatus, index int) {
    list := m.tasksByStatus[status]
    if index < 0 || index >= len(list) { return }
    m.tasksByStatus[status] = append(list[:index], list[index+1:]...)
    if index >= len(m.tasksByStatus[status]) {
        m.selectedIdx[status] = max(0, len(m.tasksByStatus[status])-1)
    }
    _ = storage.SaveTasks(m.tasksByStatus)
}

func (m model) View() string {
    // Layout
    totalWidth := max(30, m.width)
    sections := make([]string, 0, len(statusOrder))
    frameW, _ := columnStyle.GetFrameSize()
    gapW := 0
    if !m.vertical { gapW = 1 }

    if m.vertical {
        // One column per row, full width
        contentW := max(1, totalWidth - frameW)
        for _, st := range statusOrder {
            title := statusTitle(st)
            items := m.renderItems(st)
            header := headerStyle.Render(fmt.Sprintf("%s (%d)", title, len(m.tasksByStatus[st])))
            content := header + "\n" + strings.Join(items, "\n")
            style := unfocusedColStyle
            if m.focused == st { style = focusedColStyle }
            sections = append(sections, style.Width(contentW).Render(content))
        }
    } else {
        // Three columns side-by-side; balance widths exactly
        numCols := len(statusOrder)
        contentTotal := totalWidth - numCols*frameW - (numCols-1)*gapW
        if contentTotal < numCols { contentTotal = numCols }
        base := contentTotal / numCols
        rem := contentTotal - base*numCols
        widths := make([]int, numCols)
        contents := make([]string, numCols)
        styles := make([]lipgloss.Style, numCols)
        for i := 0; i < numCols; i++ {
            widths[i] = base
            if i < rem { widths[i]++ }
        }
        for i, st := range statusOrder {
            title := statusTitle(st)
            items := m.renderItems(st)
            header := headerStyle.Render(fmt.Sprintf("%s (%d)", title, len(m.tasksByStatus[st])))
            contents[i] = header + "\n" + strings.Join(items, "\n")
            style := unfocusedColStyle
            if m.focused == st { style = focusedColStyle }
            styles[i] = style
            sections = append(sections, style.Width(max(1, widths[i])).Render(contents[i]))
        }
        // Measure and correct any rounding width diff by adjusting the last column
        gap := lipgloss.NewStyle().Width(gapW).Render(" ")
        boardCandidate := lipgloss.JoinHorizontal(lipgloss.Top, interleave(sections, gap)...)
        diff := totalWidth - lipgloss.Width(boardCandidate)
        if diff != 0 {
            // expand or shrink last column to fit exactly
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

func (m model) renderItems(status domain.TaskStatus) []string {
    list := append([]domain.Task(nil), m.tasksByStatus[status]...)
    // show starred first
    sort.SliceStable(list, func(i, j int) bool {
        if list[i].IsStarred != list[j].IsStarred {
            return list[i].IsStarred
        }
        return list[i].CreatedAt > list[j].CreatedAt
    })

    // Map back indices for selection highlighting
    indexInOriginal := func(task domain.Task) int {
        for i, t := range m.tasksByStatus[status] {
            if t.Id == task.Id { return i }
        }
        return -1
    }

    lines := make([]string, 0, len(list))
    for i, t := range list {
        star := "  "
        if t.IsStarred { star = starredStyle.Render("★ ") }
        text := t.Content
        if status == domain.TaskStatusDone { text = doneStyle.Render(text) }
        isSelected := indexInOriginal(t) == m.selectedIdx[status] && m.focused == status && m.mode == modeList
        var line string
        if isSelected {
            // Keep star color; only reverse the task text
            line = fmt.Sprintf("%s %s%s", cursorBullet, star, selectedItemStyle.Render(text))
        } else {
            // Keep alignment when not selected (two-space prefix the width of the bullet+space)
            line = fmt.Sprintf("  %s%s", star, text)
        }
        lines = append(lines, line)
        _ = i // silence unused variable in case
    }
    return lines
}

// sortedOrder returns the indices of tasks in the given status, ordered the
// same as shown in the UI (starred first, then newest first).
func (m model) sortedOrder(status domain.TaskStatus) []int {
    original := m.tasksByStatus[status]
    order := make([]int, len(original))
    for i := range order { order[i] = i }
    sort.SliceStable(order, func(i, j int) bool {
        ti := original[order[i]]
        tj := original[order[j]]
        if ti.IsStarred != tj.IsStarred {
            return ti.IsStarred
        }
        return ti.CreatedAt > tj.CreatedAt
    })
    return order
}

func indexOf(slice []int, value int) int {
    for i, v := range slice {
        if v == value { return i }
    }
    return -1
}

func statusTitle(s domain.TaskStatus) string {
    switch s {
    case domain.TaskStatusTodo:
        return "Todo"
    case domain.TaskStatusInProgress:
        return "In Progress"
    case domain.TaskStatusDone:
        return "Done"
    default:
        return "Unknown"
    }
}

func prevStatus(s domain.TaskStatus) domain.TaskStatus {
    switch s {
    case domain.TaskStatusTodo:
        return domain.TaskStatusTodo
    case domain.TaskStatusInProgress:
        return domain.TaskStatusTodo
    case domain.TaskStatusDone:
        return domain.TaskStatusInProgress
    default:
        return domain.TaskStatusTodo
    }
}

func nextStatus(s domain.TaskStatus) domain.TaskStatus {
    switch s {
    case domain.TaskStatusTodo:
        return domain.TaskStatusInProgress
    case domain.TaskStatusInProgress:
        return domain.TaskStatusDone
    case domain.TaskStatusDone:
        return domain.TaskStatusDone
    default:
        return domain.TaskStatusDone
    }
}

func newID() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func max(a, b int) int { if a > b { return a }; return b }

// interleave returns a slice like: a0, sep, a1, sep, a2 ...
func interleave(items []string, sep string) []string {
    if len(items) == 0 { return items }
    out := make([]string, 0, len(items)*2-1)
    for i, s := range items {
        if i > 0 { out = append(out, sep) }
        out = append(out, s)
    }
    return out
}

// renderHelp lays out the shortcut hints across multiple columns
func (m model) renderHelp(totalWidth int) string {
    items := []string{
        "h/l: focus column",
        "j/k: move",
        "g/G: top/bottom",
        "[ \\ / ]: move task",
        "space/x: toggle done",
        "s: star",
        "n: new",
        "e: edit",
        "del: delete",
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
    // compute how many columns we can fit
    cols := totalWidth / (minColWidth + gapW)
    if cols < 1 { cols = 1 }
    if cols > maxCols { cols = maxCols }

    rows := (len(items) + cols - 1) / cols
    columns := make([]string, 0, cols)
    // width distribution
    contentTotal := totalWidth - (cols-1)*gapW
    base := contentTotal / cols
    rem := contentTotal - base*cols
    for c := 0; c < cols; c++ {
        // gather items for this column (column-major)
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
        if c < rem { w++ }
        col := footerStyle.Copy().Width(max(1, w)).Render(strings.Join(lines, "\n"))
        columns = append(columns, col)
    }
    gap := lipgloss.NewStyle().Width(gapW).Render(" ")
    return lipgloss.JoinHorizontal(lipgloss.Top, interleave(columns, gap)...)
}

func main() {
    p := tea.NewProgram(initialModel(), tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Printf("error: %v\n", err)
        os.Exit(1)
    }
}
