package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

const mediaTypeSymmetraPreview = "application/vnd.github.symmetra-preview+json"

var deleteExisting bool
var labelsFile string

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Add labels to a repo based on a json file",
	Long: `Create adds all labels specified in the provided JSON file.
The JSON file expects a format including name, color, and description.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		client := initGithubClient(ctx)

		labeller := Labeller{
			ctx:         ctx,
			client:      client,
			gitLabelMap: nil,
			tags:        tags,
		}
		labeller.CreateLabels(owner, repo)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:

	createCmd.Flags().BoolVarP(&deleteExisting, "delete", "d", true, "Delete all existing labels")
	createCmd.Flags().StringVarP(&labelsFile, "labels", "l", "labels.json", "JSON of labels to create")
}

// GitLabel represents the JSON structure of the labels defined in the labels file
type GitLabel struct {
	Name        string `json:"name,omitempty"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

// CreateLabels creates Github Labels based on the provided JSON.
func (l Labeller) CreateLabels(owner string, repo string) {
	if deleteExisting {
		labels, _, _ := l.client.Issues.ListLabels(l.ctx, owner, repo, &github.ListOptions{})
		for _, label := range labels {
			l.client.Issues.DeleteLabel(l.ctx, owner, repo, *label.Name)
		}
	}

	var labels []GitLabel
	ld, err := ioutil.ReadFile(labelsFile)
	if err != nil {
		panic(fmt.Errorf("Error occurred while reading from %v", labelsFile))
	}

	err = json.Unmarshal(ld, &labels)
	if err != nil {
		panic(fmt.Errorf("Error occurred while unmarshalling labels file"))
	}

	for _, gitLabel := range labels {
		u := fmt.Sprintf("repos/%v/%v/labels", owner, repo)
		req, _ := l.client.NewRequest("POST", u, &gitLabel)
		req.Header.Set("Accept", mediaTypeSymmetraPreview)
		l.client.Do(l.ctx, req, &gitLabel)
	}

	fmt.Printf("Added all labels from %s to %s \n", labelsFile, repo)
}
