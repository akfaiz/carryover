package components

// ConfirmOpts configures ConfirmDanger.
type ConfirmOpts struct {
	Title, Message string
	// With Check set, a checkbox (unticked) offers to delete more; Checked
	// replaces Message while it is ticked, and KeptHint is shown while it isn't.
	Check, Checked, KeptHint string
	Action                   string // the danger button's label ("Delete" if empty)
	OnConfirm                func(Checked bool)
}
