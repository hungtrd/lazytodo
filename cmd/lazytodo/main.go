package main

import (
    "fmt"
    "os"

    "github.com/hungtrd/lazytodo/internal/repository/fs"
    "github.com/hungtrd/lazytodo/internal/task"
    "github.com/hungtrd/lazytodo/internal/ui"
)

func main() {
    taskRepo := fs.NewTaskStore()
    cfgRepo := fs.NewConfigStore()
    svc := task.NewService(taskRepo, cfgRepo)
    if err := ui.Run(svc); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }
}
