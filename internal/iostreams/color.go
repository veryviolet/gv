package iostreams

import "fmt"

// ColorScheme renders semantic colours, collapsing to plain text when colour is off.
type ColorScheme struct {
	enabled bool
}

func NewColorScheme(enabled bool) *ColorScheme {
	return &ColorScheme{enabled: enabled}
}

func (c *ColorScheme) Enabled() bool { return c.enabled }

func (c *ColorScheme) wrap(code, s string) string {
	if !c.enabled {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (c *ColorScheme) Bold(s string) string    { return c.wrap("1", s) }
func (c *ColorScheme) Red(s string) string     { return c.wrap("31", s) }
func (c *ColorScheme) Green(s string) string   { return c.wrap("32", s) }
func (c *ColorScheme) Yellow(s string) string  { return c.wrap("33", s) }
func (c *ColorScheme) Blue(s string) string    { return c.wrap("34", s) }
func (c *ColorScheme) Magenta(s string) string { return c.wrap("35", s) }
func (c *ColorScheme) Cyan(s string) string    { return c.wrap("36", s) }
func (c *ColorScheme) Gray(s string) string    { return c.wrap("90", s) }

func (c *ColorScheme) Boldf(format string, a ...any) string {
	return c.Bold(fmt.Sprintf(format, a...))
}

func (c *ColorScheme) Redf(format string, a ...any) string {
	return c.Red(fmt.Sprintf(format, a...))
}

func (c *ColorScheme) Grayf(format string, a ...any) string {
	return c.Gray(fmt.Sprintf(format, a...))
}

func (c *ColorScheme) SuccessIcon() string { return c.Green("✓") }
func (c *ColorScheme) FailureIcon() string { return c.Red("✗") }
func (c *ColorScheme) WarningIcon() string { return c.Yellow("!") }

// ColorFromString maps a colour name to the matching method, for label rendering.
func (c *ColorScheme) ColorFromString(name string) func(string) string {
	switch name {
	case "red":
		return c.Red
	case "green":
		return c.Green
	case "yellow":
		return c.Yellow
	case "blue":
		return c.Blue
	case "magenta":
		return c.Magenta
	case "cyan":
		return c.Cyan
	case "gray":
		return c.Gray
	default:
		return func(s string) string { return s }
	}
}
