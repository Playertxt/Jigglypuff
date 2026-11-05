package engine

import (
	"context"
	"errors"
	"time"
)

// UninstallResult says how an uninstall went.
type UninstallResult struct {
	Status       string `json:"status"` // removed | still-installed | skipped-wait | failed
	ExitCode     int    `json:"exitCode"`
	RestorePoint string `json:"restorePoint"`
	Message      string `json:"message,omitempty"`
}

// Uninstall runs an app's own uninstaller (optionally after creating a
// restore point) and waits for it to finish. Cancelling ctx stops waiting but
// never kills the uninstaller.
func (e *Engine) Uninstall(ctx context.Context, app *App, restorePoint bool, progress ProgressFunc) UninstallResult {
	res := UninstallResult{RestorePoint: "off"}
	if restorePoint {
		if !e.P.IsAdmin() {
			res.RestorePoint = "skipped — needs administrator rights"
		} else {
			progress(Progress{Phase: "restore-point", Message: "Creating a restore point"})
			status, err := e.P.CreateRestorePoint("Jigglypuff: before uninstalling " + app.Name)
			if err != nil {
				res.RestorePoint = "failed — " + err.Error()
			} else {
				res.RestorePoint = status
			}
		}
	}
	if ctx.Err() != nil {
		res.Status = "skipped-wait"
		return res
	}
	progress(Progress{Phase: "uninstalling", Message: "Waiting for " + app.Name + "'s uninstaller"})
	switch app.Kind {
	case "store":
		if err := e.P.RemoveStoreApp(app.StoreFull); err != nil {
			res.Status, res.Message = "failed", err.Error()
			return res
		}
		res.Status = "removed"
		return res
	default:
		cmd := app.UninstallCommand()
		if cmd == "" {
			res.Status, res.Message = "failed", "This app has no uninstaller. Use Force remove instead."
			return res
		}
		code, err := e.P.RunUninstaller(ctx, cmd)
		res.ExitCode = code
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				res.Status = "skipped-wait"
				return res
			}
			res.Status, res.Message = "failed", err.Error()
			return res
		}
	}
	progress(Progress{Phase: "verifying", Message: "Checking whether " + app.Name + " is gone"})
	for i := 0; i < 12 && e.P.Reg().Exists(app.RegKey); i++ {
		select {
		case <-ctx.Done():
			res.Status = "skipped-wait"
			return res
		case <-time.After(500 * time.Millisecond):
		}
	}
	if e.P.Reg().Exists(app.RegKey) {
		res.Status = "still-installed"
		res.Message = app.Name + " still appears in the list of installed apps. The uninstaller may have been cancelled."
		return res
	}
	res.Status = "removed"
	return res
}
