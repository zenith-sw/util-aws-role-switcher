package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/atotto/clipboard"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type ProfileConfig struct {
	Profile     string            `yaml:"profile"`
	Name        string            `yaml:"name"`
	Duration    int32             `yaml:"duration"`
	AssumeRoles map[string]string `yaml:"assume_roles"`
}

var rootCmd = &cobra.Command{
	Use:   "sw",
	Short: "AWS STS Role Switcher",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".aws")
		configPath := filepath.Join(configDir, "config.yaml")

		if _, err := os.Stat(configPath); err == nil {
			fmt.Println("⚠️ Configuration file already exists:", configPath)
			return
		}

		os.MkdirAll(configDir, 0755)
		defaultConfig := []ProfileConfig{{
			Profile:  "default",
			Name:     "user",
			Duration: 3600,
			AssumeRoles: map[string]string{
				"dev": "",
				"stg": "",
				"prod": "",
				"sbx": "",
			},
		}}

		data, _ := yaml.Marshal(&defaultConfig)
		os.WriteFile(configPath, data, 0644)
		fmt.Println("✅ Configuration file already exists:", configPath)
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup [alias]",
	Short: "Get temporary credentials for a role alias",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetAlias := args[0]
		home, _ := os.UserHomeDir()
		configPath := filepath.Join(home, ".aws", "config.yaml")

		yamlData, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("❌ Unable to read settings, please run 'sw init' first: %v", err)
		}

		var profiles []ProfileConfig
		yaml.Unmarshal(yamlData, &profiles)

		var targetRoleARN string
		var duration int32 = 3600

		for _, p := range profiles {
			if arn, ok := p.AssumeRoles[targetAlias]; ok {
				targetRoleARN = arn
				duration = p.Duration
				break
			}
		}

		if targetRoleARN == "" {
			log.Fatalf("❌ '%s' role not found", targetAlias)
		}

		cfg, _ := config.LoadDefaultConfig(context.TODO())
		stsClient := sts.NewFromConfig(cfg)
		res, err := stsClient.AssumeRole(context.TODO(), &sts.AssumeRoleInput{
			RoleArn:         &targetRoleARN,
			RoleSessionName: &targetAlias,
			DurationSeconds: &duration,
		})
		if err != nil {
			log.Fatalf("❌ Role switching failed: %v", err)
		}

		output := fmt.Sprintf(
			"export AWS_ACCESS_KEY_ID=%s\nexport AWS_SECRET_ACCESS_KEY=%s\nexport AWS_SESSION_TOKEN=%s",
			*res.Credentials.AccessKeyId, *res.Credentials.SecretAccessKey, *res.Credentials.SessionToken,
		)

		clipboard.WriteAll(output)
		fmt.Printf("✅ [%s] credentials have been copied!\nPaste and export as an environment variable.", targetAlias)
	},
}

func main() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setupCmd)
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}