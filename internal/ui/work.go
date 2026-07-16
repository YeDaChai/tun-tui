package ui

// workState is the single in-flight operation flag for the TUI.
// Replaces the old busy/starting/loadingNodes boolean pile-up.
type workState int

const (
	workIdle workState = iota
	workConnecting
	workLoadingNodes
	workTesting
	workActing // mode switch, select node, link CRUD, clear data
)

func (s workState) busy() bool { return s != workIdle }

func (s workState) spinning() bool {
	return s == workConnecting || s == workLoadingNodes
}
