package main

import "html/template"

// Small inline SVG icons in the spirit of GitLab's CI status badges.
// They draw with currentColor; the colour comes from CSS classes.

const ring = `<circle cx="8" cy="8" r="6.6" fill="none" stroke="currentColor" stroke-width="1.6"/>`

var pipelineGlyphs = map[string]string{
	"success":   `<circle cx="8" cy="8" r="7.4" fill="currentColor"/><path d="M4.8 8.3l2.1 2.1 4.3-4.6" fill="none" stroke="var(--icon-on)" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>`,
	"failed":    `<circle cx="8" cy="8" r="7.4" fill="currentColor"/><path d="M5.6 5.6l4.8 4.8M10.4 5.6l-4.8 4.8" stroke="var(--icon-on)" stroke-width="1.7" stroke-linecap="round"/>`,
	"running":   ring + `<path d="M8 3.2a4.8 4.8 0 0 1 0 9.6z" fill="currentColor"/>`,
	"pending":   ring + `<path d="M6.3 5.4v5.2M9.7 5.4v5.2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>`,
	"canceled":  ring + `<path d="M5.2 10.8l5.6-5.6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>`,
	"skipped":   ring + `<path d="M5 5.5l2.5 2.5L5 10.5M8.5 5.5L11 8l-2.5 2.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>`,
	"manual":    ring + `<path d="M6.6 5.3v5.4L11 8z" fill="currentColor"/>`,
	"scheduled": ring + `<path d="M8 4.9V8l2.1 1.4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>`,
	"created":   ring + `<circle cx="8" cy="8" r="2.2" fill="currentColor"/>`,
	"waiting":   ring + `<circle cx="5.4" cy="8" r="1" fill="currentColor"/><circle cx="8" cy="8" r="1" fill="currentColor"/><circle cx="10.6" cy="8" r="1" fill="currentColor"/>`,
}

var pipelineAlias = map[string]string{
	"canceling":            "canceled",
	"preparing":            "waiting",
	"waiting_for_resource": "waiting",
	"waiting_for_callback": "waiting",
}

// pipelineClass maps any GitLab status to one of the glyph keys above.
func pipelineClass(status string) string {
	if a, ok := pipelineAlias[status]; ok {
		return a
	}
	if _, ok := pipelineGlyphs[status]; ok {
		return status
	}
	return "created"
}

func pipelineIcon(status string) template.HTML {
	return template.HTML(`<svg class="ci-icon" viewBox="0 0 16 16" aria-hidden="true">` + pipelineGlyphs[pipelineClass(status)] + `</svg>`)
}

var reviewGlyphs = map[string]string{
	"approved": `<path d="M3.6 6.2l1.7 1.7 3.2-3.6" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>`,
	"changes":  `<path d="M6 3v3.6M6 8.6v.1" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>`,
	"reviewed": `<path d="M3.5 4.5h5M3.5 7.2h3.4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>`,
}

func reviewIcon(state string) template.HTML {
	g, ok := reviewGlyphs[state]
	if !ok {
		return ""
	}
	return template.HTML(`<svg viewBox="0 0 12 12" aria-hidden="true">` + g + `</svg>`)
}
