// Package menu provides the interactive CLI menu system.
package menu

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ANSI color codes matching the original Bash script visual style.
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// MaxRetries is the maximum number of invalid input retries before returning.
const MaxRetries = 5

// Red returns text in red.
func Red(s string) string { return ColorRed + s + ColorReset }

// Green returns text in green.
func Green(s string) string { return ColorGreen + s + ColorReset }

// Yellow returns text in yellow.
func Yellow(s string) string { return ColorYellow + s + ColorReset }

// Cyan returns text in cyan (sky blue).
func Cyan(s string) string { return ColorCyan + s + ColorReset }

// White returns text in white.
func White(s string) string { return ColorWhite + s + ColorReset }

// Bold returns text in bold.
func Bold(s string) string { return ColorBold + s + ColorReset }

// PrintTitle prints a menu title with decorative borders.
func PrintTitle(title string) {
	line := strings.Repeat("═", 50)
	fmt.Printf("\n%s%s%s\n", Cyan("╔"), Cyan(line), Cyan("╗"))
	fmt.Printf("%s  %-48s%s\n", Cyan("║"), Bold(title), Cyan("║"))
	fmt.Printf("%s%s%s\n\n", Cyan("╚"), Cyan(line), Cyan("╝"))
}

// PrintOption prints a numbered menu option.
func PrintOption(num int, text string) {
	fmt.Printf("  %s. %s\n", Green(fmt.Sprintf("%2d", num)), text)
}

// PrintOptionStr prints a menu option with a string key.
func PrintOptionStr(key, text string) {
	fmt.Printf("  %s. %s\n", Green(key), text)
}

// PrintInfo prints an info message.
func PrintInfo(msg string) {
	fmt.Printf("  %s %s\n", Cyan("ℹ"), msg)
}

// PrintSuccess prints a success message.
func PrintSuccess(msg string) {
	fmt.Printf("  %s %s\n", Green("✓"), msg)
}

// PrintError prints an error message.
func PrintError(msg string) {
	fmt.Printf("  %s %s\n", Red("✗"), msg)
}

// PrintWarning prints a warning message.
func PrintWarning(msg string) {
	fmt.Printf("  %s %s\n", Yellow("⚠"), msg)
}

// PrintSeparator prints a horizontal separator line.
func PrintSeparator() {
	fmt.Println(strings.Repeat("─", 54))
}

// ReadInput reads a line of input from the user with a prompt.
func ReadInput(prompt string) string {
	fmt.Printf("  %s: ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// ReadChoice reads a menu choice with retry logic (no recursion).
func ReadChoice(prompt string, validChoices []string) string {
	valid := make(map[string]bool)
	for _, c := range validChoices {
		valid[c] = true
	}

	for i := 0; i < MaxRetries; i++ {
		input := ReadInput(prompt)
		if valid[input] {
			return input
		}
		if input == "0" || input == "q" || input == "" {
			return "0" // Return to parent menu.
		}
		PrintError(fmt.Sprintf("无效选择: %s", input))
	}

	PrintWarning("超过最大重试次数，返回上级菜单")
	return "0"
}

// Confirm asks for yes/no confirmation.
func Confirm(prompt string) bool {
	input := ReadInput(prompt + " [y/N]")
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}
