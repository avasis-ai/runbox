package shellquote

import "strings"

func Join(parts ...string) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(Quote(p))
	}
	return b.String()
}

func Quote(s string) string {
	if s == "" {
		return "''"
	}
	needsQuoting := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '"', '\'', '\\', '$', '`', '!', '*', '?', '[', ']', '(', ')', '{', '}', ';', '&', '|', '<', '>', '~', '#', '=', '\n', '\r':
			needsQuoting = true
		}
		if needsQuoting {
			break
		}
	}
	if !needsQuoting {
		return s
	}
	return singleQuote(s)
}

func singleQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		if r == '\'' {
			b.WriteString(`'\''`)
		} else {
			b.WriteRune(r)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func WrapForSSH(workdir, command string) string {
	if workdir == "" {
		return "bash -lc " + Quote(command)
	}
	combined := "cd " + Quote(workdir) + " && bash -lc " + Quote(command)
	return combined
}
