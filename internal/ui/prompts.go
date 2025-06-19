package ui

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
)

// Colors for consistent output
var (
	Success = color.New(color.FgGreen).SprintFunc()
	Error   = color.New(color.FgRed).SprintFunc()
	Warning = color.New(color.FgYellow).SprintFunc()
	Info    = color.New(color.FgCyan).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
)

// Confirm asks for user confirmation
func Confirm(message string, defaultValue bool) (bool, error) {
	var result bool
	prompt := &survey.Confirm{
		Message: message,
		Default: defaultValue,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// Select presents a selection list
func Select(message string, options []string) (string, error) {
	var result string
	prompt := &survey.Select{
		Message: message,
		Options: options,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// SelectWithHelp presents a selection list with help text
func SelectWithHelp(message string, options []string, help string) (string, error) {
	var result string
	prompt := &survey.Select{
		Message: message,
		Options: options,
		Help:    help,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// MultiSelect presents a multi-selection list
func MultiSelect(message string, options []string) ([]string, error) {
	var result []string
	prompt := &survey.MultiSelect{
		Message: message,
		Options: options,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// Input asks for text input
func Input(message, defaultValue string, validator survey.Validator) (string, error) {
	var result string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
	}

	opts := []survey.AskOpt{}
	if validator != nil {
		opts = append(opts, survey.WithValidator(validator))
	}

	err := survey.AskOne(prompt, &result, opts...)
	return result, err
}

// InputWithHelp asks for text input with help text
func InputWithHelp(message, defaultValue, help string, validator survey.Validator) (string, error) {
	var result string
	prompt := &survey.Input{
		Message: message,
		Default: defaultValue,
		Help:    help,
	}

	opts := []survey.AskOpt{}
	if validator != nil {
		opts = append(opts, survey.WithValidator(validator))
	}

	err := survey.AskOne(prompt, &result, opts...)
	return result, err
}

// Password asks for password input
func Password(message string) (string, error) {
	var result string
	prompt := &survey.Password{
		Message: message,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// PasswordWithHelp asks for password input with help text
func PasswordWithHelp(message, help string) (string, error) {
	var result string
	prompt := &survey.Password{
		Message: message,
		Help:    help,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// Editor opens a text editor for multi-line input
func Editor(message, defaultValue string) (string, error) {
	var result string
	prompt := &survey.Editor{
		Message: message,
		Default: defaultValue,
	}

	err := survey.AskOne(prompt, &result)
	return result, err
}

// Required is a validator that ensures input is not empty
func Required(val interface{}) error {
	if str, ok := val.(string); !ok || str == "" {
		return fmt.Errorf("this field is required")
	}
	return nil
}
