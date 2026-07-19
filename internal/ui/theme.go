package ui

import "github.com/charmbracelet/lipgloss"

// 国风修仙：宣纸夜色 + 描金 + 松绿 + 朱砂。
var (
	accent  = lipgloss.Color("#D4A84B") // 描金
	suggest = lipgloss.Color("#F0E0B0") // 浅金选中
	ok      = lipgloss.Color("#6B9B6E") // 松绿（用量健康）
	flow    = lipgloss.Color("#6BA8A0") // 仙青（下行）
	warn    = lipgloss.Color("#C9953A") // 赭黄
	danger  = lipgloss.Color("#C0453A") // 朱砂
	muted   = lipgloss.Color("#9A8F7E") // 淡墨
	subtle  = lipgloss.Color("#5A5044") // 阵纹
	selBg   = lipgloss.Color("#1C1812") // 焦墨底

	frameBorderActive = lipgloss.NewStyle().Foreground(accent)
	statusOnline      = lipgloss.NewStyle().Foreground(ok).Bold(true)
	statusOffline     = lipgloss.NewStyle().Foreground(muted)
	statusLoading     = lipgloss.NewStyle().Foreground(warn).Bold(true)
	textErr           = lipgloss.NewStyle().Foreground(danger)
	textSubtle        = lipgloss.NewStyle().Foreground(muted)
	itemSelected      = lipgloss.NewStyle().
				Foreground(suggest).
				Background(selBg).
				Bold(true)
	itemCurrent  = lipgloss.NewStyle().Foreground(ok).Bold(true)
	itemNormal   = lipgloss.NewStyle().Foreground(muted)
	pingFast     = lipgloss.NewStyle().Foreground(ok)
	pingMid      = lipgloss.NewStyle().Foreground(warn)
	pingSlow     = lipgloss.NewStyle().Foreground(danger)
	txColor      = lipgloss.NewStyle().Foreground(accent) // 上行描金
	rxColor      = lipgloss.NewStyle().Foreground(flow)   // 下行仙青
	dividerStyle = lipgloss.NewStyle().Foreground(subtle)
	leaderDim    = dividerStyle
	leaderActive = lipgloss.NewStyle().Foreground(suggest).Background(selBg)
	leaderOn     = lipgloss.NewStyle().Foreground(ok)
	footerKey    = lipgloss.NewStyle().Foreground(accent).Bold(true)
	footerSep    = lipgloss.NewStyle().Foreground(subtle)
	sectionTitle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	barFull      = lipgloss.NewStyle().Foreground(ok)
	barWarning   = lipgloss.NewStyle().Foreground(warn)
	barDanger    = lipgloss.NewStyle().Foreground(danger)
	modeActive   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	btnBorder    = lipgloss.NewStyle().Foreground(accent)
	btnLabel     = lipgloss.NewStyle().Foreground(suggest)
)
