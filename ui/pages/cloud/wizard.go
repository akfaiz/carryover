package cloud

import (
	"fmt"
	"strings"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
)

// WizardBackend is what the wizard needs of the cloud service: rclone's
// config state machine.
type WizardBackend interface {
	CreateRemote(name, typ string, params map[string]string) (*cloud.Question, error)
	ContinueRemote(name, state, answer string) (*cloud.Question, error)
}

// StepRunner does a step off the UI thread and then calls done on it.
type StepRunner func(work func() (*cloud.Question, error), done func(*cloud.Question, error))

// ResultKind says which screen follows a step.
type ResultKind int

const (
	ResultForm     ResultKind = iota // back to the options form, with an error
	ResultQuestion                   // rclone's next question (or the one just asked again, with an error)
	ResultCreated                    // the remote exists
)

// Result is what a step of the wizard came to.
type Result struct {
	Kind ResultKind
	Err  string // for ResultForm and ResultQuestion; "" if none
	Name string // the new remote, for ResultCreated
}

// Wizard creates a remote: pick a provider, fill in its options, then answer
// rclone's follow-up questions (OAuth and so on).
type Wizard struct {
	be        WizardBackend
	providers []cloud.Provider
	exists    func(name string) bool
	prov      *cloud.Provider
	name      string
	params    map[string]string
	showAll   bool
	question  *cloud.Question
}

// NewWizard returns a wizard offering providers. exists says whether a remote
// of that name is already configured.
func NewWizard(be WizardBackend, providers []cloud.Provider, exists func(string) bool) *Wizard {
	return &Wizard{be: be, providers: providers, exists: exists}
}

// Providers are the storage providers to choose from.
func (w *Wizard) Providers() []cloud.Provider { return w.providers }

// DefaultName is a name for a new remote of a provider type that no existing
// remote has: the type (gdrive for Google Drive), then type2, type3...
func DefaultName(typ string, exists func(string) bool) string {
	name := typ
	if typ == "drive" {
		name = "gdrive"
	}
	for i := 2; exists(name); i++ {
		name = fmt.Sprintf("%s%d", typ, i)
	}
	return name
}

// Choose picks a provider. Choosing the one already chosen (going back and
// forth) keeps what was typed.
func (w *Wizard) Choose(pr cloud.Provider) {
	if w.prov != nil && w.prov.Type == pr.Type {
		return
	}
	w.prov = &pr
	w.name = DefaultName(pr.Type, w.exists)
	w.params = map[string]string{}
	w.showAll = false
}

// Provider is the chosen provider; Choose must have been called.
func (w *Wizard) Provider() cloud.Provider { return *w.prov }

// Name is the remote's name as typed, SetName changes it, and NameOK reports
// whether it can be used (not blank).
func (w *Wizard) Name() string          { return w.name }
func (w *Wizard) SetName(v string)      { w.name = v }
func (w *Wizard) NameOK() bool          { return strings.TrimSpace(w.name) != "" }
func (w *Wizard) Param(n string) string { return w.params[n] }
func (w *Wizard) SetParam(n, v string)  { w.params[n] = v }

// ShowAll turns on the options that are normally hidden.
func (w *Wizard) ShowAll() { w.showAll = true }

// VisibleOptions are the options the form shows: the required ones, the
// provider choice and options that are a pick from a fixed list, or all of them
// after ShowAll. more reports that some are hidden.
func (w *Wizard) VisibleOptions() (opts []cloud.ProviderOption, more bool) {
	for _, o := range w.prov.Options {
		exclusive := len(o.Examples) > 0 && o.Exclusive
		if !w.showAll && !o.Required && o.Name != "provider" && !exclusive {
			continue
		}
		opts = append(opts, o)
	}
	return opts, len(w.prov.Options) > len(opts)
}

// Question is rclone's current question, nil before the first.
func (w *Wizard) Question() *cloud.Question { return w.question }

func (w *Wizard) step(run StepRunner, work func() (*cloud.Question, error), done func(Result)) {
	run(work, func(q *cloud.Question, err error) {
		switch {
		case err != nil && w.question != nil:
			done(Result{Kind: ResultQuestion, Err: err.Error()})
		case err != nil:
			done(Result{Kind: ResultForm, Err: err.Error()})
		case q == nil || q.Done:
			done(Result{Kind: ResultCreated, Name: strings.TrimSpace(w.name)})
		default:
			w.question = q
			done(Result{Kind: ResultQuestion, Err: q.Error})
		}
	})
}

// Create starts configuring the remote with the options as filled in.
func (w *Wizard) Create(run StepRunner, done func(Result)) {
	name, typ, params := strings.TrimSpace(w.name), w.prov.Type, w.params
	w.step(run, func() (*cloud.Question, error) { return w.be.CreateRemote(name, typ, params) }, done)
}

// Reply answers rclone's current question.
func (w *Wizard) Reply(answer string, run StepRunner, done func(Result)) {
	name, state := strings.TrimSpace(w.name), w.question.State
	w.step(run, func() (*cloud.Question, error) { return w.be.ContinueRemote(name, state, answer) }, done)
}
