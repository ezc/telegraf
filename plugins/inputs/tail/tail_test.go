package tail

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/influxdata/telegraf/plugins/parsers"
	"github.com/influxdata/telegraf/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailFromBeginning(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	_, err = tmpfile.WriteString("cpu,mytag=foo usage_idle=100\n")
	require.NoError(t, err)

	tt := NewTail()
	tt.FromBeginning = true
	tt.Files = []string{tmpfile.Name()}
	p, _ := parsers.NewInfluxParser()
	tt.SetParser(p)
	defer tt.Stop()
	defer tmpfile.Close()

	acc := testutil.Accumulator{}
	require.NoError(t, tt.Start(&acc))
	require.NoError(t, tt.Gather(&acc))

	acc.Wait(1)
	acc.AssertContainsTaggedFields(t, "cpu",
		map[string]interface{}{
			"usage_idle": float64(100),
		},
		map[string]string{
			"mytag": "foo",
		})
}

func TestTailFromEnd(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())
	_, err = tmpfile.WriteString("cpu,mytag=foo usage_idle=100\n")
	require.NoError(t, err)

	tt := NewTail()
	tt.Files = []string{tmpfile.Name()}
	p, _ := parsers.NewInfluxParser()
	tt.SetParser(p)
	defer tt.Stop()
	defer tmpfile.Close()

	acc := testutil.Accumulator{}
	require.NoError(t, tt.Start(&acc))
	for _, tailer := range tt.tailers {
		for n, err := tailer.Tell(); err == nil && n == 0; n, err = tailer.Tell() {
			// wait for tailer to jump to end
			runtime.Gosched()
		}
	}

	_, err = tmpfile.WriteString("cpu,othertag=foo usage_idle=100\n")
	require.NoError(t, err)
	require.NoError(t, tt.Gather(&acc))

	acc.Wait(1)
	acc.AssertContainsTaggedFields(t, "cpu",
		map[string]interface{}{
			"usage_idle": float64(100),
		},
		map[string]string{
			"othertag": "foo",
		})
	assert.Len(t, acc.Metrics, 1)
}

func TestTailBadLine(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	tt := NewTail()
	tt.FromBeginning = true
	tt.Files = []string{tmpfile.Name()}
	p, _ := parsers.NewInfluxParser()
	tt.SetParser(p)
	defer tt.Stop()
	defer tmpfile.Close()

	acc := testutil.Accumulator{}
	require.NoError(t, tt.Start(&acc))

	_, err = tmpfile.WriteString("cpu mytag= foo usage_idle= 100\n")
	require.NoError(t, err)
	require.NoError(t, tt.Gather(&acc))

	acc.WaitError(1)
	assert.Contains(t, acc.Errors[0].Error(), "E! Malformed log line")
}
