package capicmd_test

import (
	"bytes"
	"testing"

	"github.com/davidnix/capi/internal/capicmd"
	"github.com/stretchr/testify/require"
)

func TestNewRootCommand_SilencesCobraOutputOnError(t *testing.T) {
	t.Parallel()

	cmd := capicmd.NewRootCommand()
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"googleads", "validate"})

	err := cmd.ExecuteContext(t.Context())

	require.EqualError(t, err, "missing required flags: --customer-id, --conversion-action-id, --developer-token, --oauth-client-id, --oauth-client-secret, --oauth-refresh-token, --email")
	require.Empty(t, stdout.String())
	require.Empty(t, stderr.String())
}
