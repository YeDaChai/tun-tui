package ui

import "github.com/charmbracelet/lipgloss"

// Color palette — clean, calm terminal tool
var (
	accent = lipgloss.Color("39")  // soft blue
	ok     = lipgloss.Color("71")  // soft green
	warn   = lipgloss.Color("178") // soft yellow
	danger = lipgloss.Color("167") // soft red

	fg     = lipgloss.Color("252")
	muted  = lipgloss.Color("245")
	subtle = lipgloss.Color("240")
	selBg  = lipgloss.Color("236")
)

var (
	frameBorderInactive = lipgloss.NewStyle().Foreground(subtle)
	frameBorderActive   = lipgloss.NewStyle().Foreground(accent)

	statusOnline  = lipgloss.NewStyle().Foreground(ok).Bold(true)
	statusOffline = lipgloss.NewStyle().Foreground(muted)
	statusLoading = lipgloss.NewStyle().Foreground(warn).Bold(true)

	textErr    = lipgloss.NewStyle().Foreground(danger)
	textMuted  = lipgloss.NewStyle().Foreground(muted)
	textSubtle = lipgloss.NewStyle().Foreground(subtle)
	textBody   = lipgloss.NewStyle().Foreground(fg)

	inputPanel = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(accent).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(false).
			BorderRight(false).
			Padding(1, 0)

	itemSelected = lipgloss.NewStyle().
			Foreground(accent).
			Background(selBg).
			Bold(true)
	itemCurrent = lipgloss.NewStyle().
			Foreground(ok).
			Bold(true)
	itemNormal = lipgloss.NewStyle().
			Foreground(muted)

	pingFast = lipgloss.NewStyle().Foreground(ok)
	pingMid  = lipgloss.NewStyle().Foreground(warn)
	pingSlow = lipgloss.NewStyle().Foreground(danger)
	pingDead = lipgloss.NewStyle().Foreground(muted)

	txColor = lipgloss.NewStyle().Foreground(accent)
	rxColor = lipgloss.NewStyle().Foreground(ok)

	dividerStyle = lipgloss.NewStyle().Foreground(subtle)

	footerKey   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	footerLabel = lipgloss.NewStyle().Foreground(muted)
	footerSep   = lipgloss.NewStyle().Foreground(subtle)

	sectionTitle = lipgloss.NewStyle().
			Foreground(accent).
			Bold(true)

	barFull    = lipgloss.NewStyle().Foreground(ok)
	barWarning = lipgloss.NewStyle().Foreground(warn)
	barDanger  = lipgloss.NewStyle().Foreground(danger)
	barEmpty   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	modeActive = lipgloss.NewStyle().Foreground(accent).Bold(true)

	rulesOn  = lipgloss.NewStyle().Foreground(ok).Bold(true)
	rulesOff = lipgloss.NewStyle().Foreground(muted)
	rulesBad = lipgloss.NewStyle().Foreground(warn).Bold(true)
)
