package kit

// ToastKind is what a toast says: information, success or an error.
type ToastKind int

const (
	ToastInfo ToastKind = iota
	ToastOK
	ToastErr
)
