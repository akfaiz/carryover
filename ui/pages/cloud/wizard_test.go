package cloud

import (
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/cloudsync/cloud"
)

type fakeWizardBackend struct {
	created   []string
	continued []string
	q         *cloud.Question
	err       error
}

func (f *fakeWizardBackend) CreateRemote(name, typ string, params map[string]string) (*cloud.Question, error) {
	f.created = append(f.created, name+":"+typ)
	return f.q, f.err
}
func (f *fakeWizardBackend) ContinueRemote(name, state, answer string) (*cloud.Question, error) {
	f.continued = append(f.continued, name+":"+state+":"+answer)
	return f.q, f.err
}

func syncRun(work func() (*cloud.Question, error), done func(*cloud.Question, error)) {
	q, err := work()
	done(q, err)
}

func taken(names ...string) func(string) bool {
	return func(n string) bool { return slices.Contains(names, n) }
}

func TestDefaultName(t *testing.T) {
	assert.Equal(t, "dropbox", DefaultName("dropbox", taken()))
	assert.Equal(t, "gdrive", DefaultName("drive", taken()), "Google Drive is called gdrive")
	assert.Equal(t, "dropbox2", DefaultName("dropbox", taken("dropbox")))
	assert.Equal(t, "dropbox3", DefaultName("dropbox", taken("dropbox", "dropbox2")))
	assert.Equal(t, "drive2", DefaultName("drive", taken("gdrive")), "the numbered ones follow the type")
}

func TestChooseKeepsWhatWasTypedWhenGoingBackAndForth(t *testing.T) {
	w := NewWizard(&fakeWizardBackend{}, nil, taken())
	w.Choose(cloud.Provider{Type: "sftp"})
	w.SetName("my-nas")
	w.SetParam("host", "nas.local")
	w.ShowAll()

	w.Choose(cloud.Provider{Type: "sftp"}) // back to the list, the same one again
	assert.Equal(t, "my-nas", w.Name())
	assert.Equal(t, "nas.local", w.Param("host"))

	w.Choose(cloud.Provider{Type: "webdav"})
	assert.Equal(t, "webdav", w.Name(), "another provider starts afresh")
	assert.Empty(t, w.Param("host"))
	_, more := w.VisibleOptions()
	assert.False(t, more)
}

func TestNameOK(t *testing.T) {
	w := NewWizard(&fakeWizardBackend{}, nil, taken())
	w.Choose(cloud.Provider{Type: "sftp"})
	assert.True(t, w.NameOK())
	w.SetName("   ")
	assert.False(t, w.NameOK())
}

func TestVisibleOptions(t *testing.T) {
	w := NewWizard(&fakeWizardBackend{}, nil, taken())
	w.Choose(cloud.Provider{Type: "s", Options: []cloud.ProviderOption{
		{Name: "host", Required: true},
		{Name: "provider"},
		{Name: "mode", Exclusive: true, Examples: []cloud.OptionExample{{Value: "a"}}},
		{Name: "free text examples", Examples: []cloud.OptionExample{{Value: "a"}}}, // not exclusive
		{Name: "chunk_size"},
	}})

	opts, more := w.VisibleOptions()
	names := []string{}
	for _, o := range opts {
		names = append(names, o.Name)
	}
	assert.Equal(t, []string{"host", "provider", "mode"}, names, "required, the provider choice, and pick-from-a-list")
	assert.True(t, more)

	w.ShowAll()
	opts, more = w.VisibleOptions()
	assert.Len(t, opts, 5)
	assert.False(t, more)
}

func TestCreateThenAnswerRcloneQuestionsUntilDone(t *testing.T) {
	be := &fakeWizardBackend{q: &cloud.Question{State: "s1", Help: "Use auto config?"}}
	w := NewWizard(be, nil, taken())
	w.Choose(cloud.Provider{Type: "drive"})
	w.SetName("  gdrive  ")

	var res Result
	w.Create(syncRun, func(r Result) { res = r })
	assert.Equal(t, []string{"gdrive:drive"}, be.created, "the name is trimmed")
	assert.Equal(t, Result{Kind: ResultQuestion}, res)
	require.NotNil(t, w.Question())
	assert.Equal(t, "s1", w.Question().State)

	be.q = &cloud.Question{State: "s2", Error: "bad token"}
	w.Reply("true", syncRun, func(r Result) { res = r })
	assert.Equal(t, []string{"gdrive:s1:true"}, be.continued, "answers the question that was asked")
	assert.Equal(t, Result{Kind: ResultQuestion, Err: "bad token"}, res, "a question that comes with an error shows it")

	be.q = &cloud.Question{Done: true}
	w.Reply("x", syncRun, func(r Result) { res = r })
	assert.Equal(t, Result{Kind: ResultCreated, Name: "gdrive"}, res)
}

func TestARemoteWithNoQuestionsIsCreatedAtOnce(t *testing.T) {
	w := NewWizard(&fakeWizardBackend{q: nil}, nil, taken())
	w.Choose(cloud.Provider{Type: "sftp"})
	var res Result
	w.Create(syncRun, func(r Result) { res = r })
	assert.Equal(t, Result{Kind: ResultCreated, Name: "sftp"}, res)
}

// An error before any question goes back to the form; one during the
// questions stays on the question.
func TestWhereAnErrorGoes(t *testing.T) {
	be := &fakeWizardBackend{err: errors.New("name exists")}
	w := NewWizard(be, nil, taken())
	w.Choose(cloud.Provider{Type: "sftp"})

	var res Result
	w.Create(syncRun, func(r Result) { res = r })
	assert.Equal(t, Result{Kind: ResultForm, Err: "name exists"}, res)

	be.err = nil
	be.q = &cloud.Question{State: "s1"}
	w.Create(syncRun, func(r Result) { res = r })
	be.err = errors.New("timed out")
	w.Reply("x", syncRun, func(r Result) { res = r })
	assert.Equal(t, Result{Kind: ResultQuestion, Err: "timed out"}, res)
}
