package msg_test

import (
	"encoding/xml"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akfaiz/carryover/internal/app/msg"
)

const androidMain = "../../../android/app/src/main/"

var (
	placeholder = regexp.MustCompile(`\{\{\.(\w+)\}\}`)
	coreEntry   = regexp.MustCompile(`"([\w.]+)" to CoreString\.(Text|Plural)\(R\.(?:string|plurals)\.(\w+)((?:, "\w+")*)\),`)
	positional  = regexp.MustCompile(`%(\d+)\$s`)
)

// argNames are the placeholders of a template in the order they first appear.
func argNames(template string) []string {
	var out []string
	for _, m := range placeholder.FindAllStringSubmatch(template, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// unescape reads the text of an Android string resource.
func unescape(s string) string {
	s = strings.NewReplacer(`\\`, `\`, `\'`, `'`, `\"`, `"`, `\n`, "\n", "\\u0020", " ", `\@`, "@", `\?`, "?").Replace(s)
	return strings.ReplaceAll(s, "%%", "%")
}

// androidTexts are the texts of a strings.xml by resource name, a plural's
// forms as "name/quantity".
func androidTexts(t *testing.T, folder string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(androidMain + "res/" + folder + "/strings.xml")
	require.NoError(t, err)
	var res struct {
		Strings []struct {
			Name string `xml:"name,attr"`
			Text string `xml:",chardata"`
		} `xml:"string"`
		Plurals []struct {
			Name  string `xml:"name,attr"`
			Items []struct {
				Quantity string `xml:"quantity,attr"`
				Text     string `xml:",chardata"`
			} `xml:"item"`
		} `xml:"plurals"`
	}
	require.NoError(t, xml.Unmarshal(data, &res))
	out := map[string]string{}
	for _, s := range res.Strings {
		out[s.Name] = s.Text
	}
	for _, p := range res.Plurals {
		for _, i := range p.Items {
			out[p.Name+"/"+i.Quantity] = i.Text
		}
	}
	return out
}

// The Android app translates every message of the catalog: CoreMessages.kt maps
// its code to a string resource, whose English is the core's.
// (CatalogTest on the Android side checks the other languages against it.)
func TestAndroidCatalog(t *testing.T) {
	data, err := os.ReadFile(androidMain + "java/com/akfaiz/carryover/data/CoreMessages.kt")
	require.NoError(t, err)
	entries := map[string][]string{}
	for _, m := range coreEntry.FindAllStringSubmatch(string(data), -1) {
		_, twice := entries[m[1]]
		assert.False(t, twice, "%s is listed twice", m[1])
		entries[m[1]] = m[2:]
	}
	english := androidTexts(t, "values")

	// resource is the English template written with the resource's %1$s.
	resource := func(template, text string, names []string) string {
		return positional.ReplaceAllStringFunc(text, func(p string) string {
			i, _ := strconv.Atoi(positional.FindStringSubmatch(p)[1])
			if i < 1 || i > len(names) {
				return p
			}
			return "{{." + names[i-1] + "}}"
		})
	}
	for _, d := range msg.Defs() {
		e, ok := entries[string(d.Code)]
		if !assert.True(t, ok, "%s is missing from CoreMessages.kt", d.Code) {
			continue
		}
		delete(entries, string(d.Code))
		kind, name := e[0], e[1]
		names := argNames(d.Other)
		listed := regexp.MustCompile(`\w+`).FindAllString(e[2], -1)
		assert.Equal(t, names, listed, "placeholders of %s in CoreMessages.kt", d.Code)
		forms := map[string]string{"": d.Other}
		wantKind := "Text"
		if d.One != "" {
			forms, wantKind = map[string]string{"/one": d.One, "/other": d.Other}, "Plural"
		}
		assert.Equal(t, wantKind, kind, "kind of %s", d.Code)
		for form, template := range forms {
			text, ok := english[name+form]
			if assert.True(t, ok, "%s%s is missing from values/strings.xml", name, form) {
				assert.Equal(t, template, resource(template, unescape(text), names), "English of %s%s", name, form)
			}
		}
	}
	assert.Empty(t, entries, "codes in CoreMessages.kt that the core doesn't have")
}
