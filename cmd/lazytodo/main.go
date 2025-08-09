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
    selectedItemStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("229"))
    starredStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
    doneStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Strikethrough(true)
    footerStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1)
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
        if cur > 0 { m.selectedIdx[col] = cur - 1 }
    case "down", "j":
        if len(items) == 0 { return m, nil }
        if cur < len(items)-1 { m.selectedIdx[col] = cur + 1 }
    case "left", "h":
        m.focused = prevStatus(m.focused)
    case "right", "l":
        m.focused = nextStatus(m.focused)
    case "[":
        m = m.moveTask(col, cur, prevStatus(col))
    case "]":
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
        m.selectedIdx[col] = 0
    case "G":
        if len(items) > 0 { m.selectedIdx[col] = len(items) - 1 }
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
    usableWidth := max(30, m.width-4)
    colWidth := usableWidth / 3
    sections := make([]string, 0, 3)
    for _, st := range statusOrder {
        title := statusTitle(st)
        items := m.renderItems(st)
        header := headerStyle.Render(fmt.Sprintf("%s (%d)", title, len(m.tasksByStatus[st])))
        content := header + "\n" + strings.Join(items, "\n")
        style := unfocusedColStyle
        if m.focused == st { style = focusedColStyle }
        sections = append(sections, style.Width(colWidth).Render(content))
    }
    board := lipgloss.JoinHorizontal(lipgloss.Top, sections...)

    help := footerStyle.Render("h/l: focus column  j/k: move  [ / ]: move task  n: new  e: edit  s: star  space/x: toggle done  del: delete  q: quit  esc: cancel")

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
        line := fmt.Sprintf("%s%s", star, text)
        // highlight if this is the selected item in original order
        if indexInOriginal(t) == m.selectedIdx[status] && m.focused == status && m.mode == modeList {
            line = selectedItemStyle.Render(line)
        }
        // truncate to column width - padding best effort
        lines = append(lines, line)
        _ = i // silence unused variable in case
    }
    return lines
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

func main() {
    p := tea.NewProgram(initialModel(), tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Printf("error: %v\n", err)
        os.Exit(1)
    }
}
