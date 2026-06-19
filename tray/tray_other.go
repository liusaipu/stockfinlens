//go:build !darwin

package tray

import "context"

func initTray(ctx context.Context)                        {}
func quitTray()                                           {}
func updateTrayTitle(title string, changePercent float64) {}
func setTrayQuotesJSON(json string)                       {}
func setTrayScrollEnabled(enabled bool)                   {}
func setTrayIconVisible(visible bool)                     {}
func isTrayScrollEnabled() bool                           { return true }
func isTrayIconVisible() bool                             { return true }
