package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "pactl")
	assert.Contains(t, output, "Polyant")
}

func TestVersionFlag(t *testing.T) {
	// Capture stdout since the version subcommand uses fmt.Printf
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"version"})

	err = rootCmd.Execute()
	require.NoError(t, err)

	w.Close()
	os.Stdout = old

	var stdout bytes.Buffer
	_, _ = stdout.ReadFrom(r)
	output := stdout.String()
	assert.Contains(t, output, version)
}

func TestDefaultServerAddress(t *testing.T) {
	serverAddr = "http://localhost:8080"
	assert.Equal(t, "http://localhost:8080", serverAddr)
}

func TestClientCreation(t *testing.T) {
	c := NewClient("http://localhost:8080")
	assert.NotNil(t, c)
	assert.Equal(t, "http://localhost:8080", c.baseURL)
}
