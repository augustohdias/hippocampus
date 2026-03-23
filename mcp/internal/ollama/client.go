package ollama

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Client struct {
	host  string
	model string
}

func NewClient(host, model string) *Client {
	return &Client{
		host:  host,
		model: model,
	}
}

func (c *Client) GetEmbedding(text string) ([]float32, error) {
	cmd := exec.Command("ollama", "run", c.model, text)
	if c.host != "" {
		cmd.Env = append(os.Environ(), "OLLAMA_HOST="+c.host)
	}

	output, err := cmd.Output()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			return nil, fmt.Errorf("ollama command failed: %s, stderr: %s", err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("failed to execute ollama command: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))

	embedding, err := parseEmbeddingOutput(outputStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ollama embedding output: %w, output: %s", err, outputStr)
	}

	if len(embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned from ollama")
	}

	return embedding, nil
}

func parseEmbeddingOutput(output string) ([]float32, error) {
	output = strings.TrimSpace(output)

	if strings.HasPrefix(output, "[") && strings.HasSuffix(output, "]") {
		output = output[1 : len(output)-1]
	}

	parts := strings.Split(output, ",")
	embedding := make([]float32, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		val, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to parse embedding value '%s': %w", part, err)
		}
		embedding = append(embedding, float32(val))
	}

	return embedding, nil
}
