package main

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parseHTML(s string) *html.Node {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		panic(err)
	}
	return doc
}

func TestConvertHeadings(t *testing.T) {
	h := `<h1>Heading 1</h1>
<h2>Heading 2</h2>
<h3>Heading 3</h3>
<h4>Heading 4</h4>
<h5>Heading 5</h5>
<h6>Heading 6</h6>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	tests := []string{
		"# Heading 1",
		"## Heading 2",
		"### Heading 3",
		"#### Heading 4",
		"##### Heading 5",
		"###### Heading 6",
	}
	for _, want := range tests {
		if !strings.Contains(result, want) {
			t.Errorf("expected %q in output, got:\n%s", want, result)
		}
	}
}

func TestConvertParagraphs(t *testing.T) {
	h := `<p>First paragraph.</p><p>Second paragraph.</p>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "First paragraph.") {
		t.Errorf("missing first paragraph in:\n%s", result)
	}
	if !strings.Contains(result, "Second paragraph.") {
		t.Errorf("missing second paragraph in:\n%s", result)
	}
	// Should have a blank line between paragraphs
	if !strings.Contains(result, "\n\n") {
		t.Errorf("expected blank line between paragraphs in:\n%s", result)
	}
}

func TestConvertCodeBlocks(t *testing.T) {
	h := `<pre><code class="language-go">fmt.Println("hello")</code></pre>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "```go") {
		t.Errorf("expected ```go fenced block in:\n%s", result)
	}
	if !strings.Contains(result, `fmt.Println("hello")`) {
		t.Errorf("expected code content in:\n%s", result)
	}
	if !strings.Contains(result, "```") {
		t.Errorf("expected closing ``` in:\n%s", result)
	}
}

func TestConvertInlineCode(t *testing.T) {
	h := `<p>Call <code>fmt.Println</code> here.</p>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "`fmt.Println`") {
		t.Errorf("expected inline code `fmt.Println` in:\n%s", result)
	}
}

func TestConvertUnorderedList(t *testing.T) {
	h := `<ul><li>Apple</li><li>Banana</li><li>Cherry</li></ul>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	for _, item := range []string{"- Apple", "- Banana", "- Cherry"} {
		if !strings.Contains(result, item) {
			t.Errorf("expected %q in:\n%s", item, result)
		}
	}
}

func TestConvertOrderedList(t *testing.T) {
	h := `<ol><li>First</li><li>Second</li><li>Third</li></ol>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	for _, item := range []string{"1. First", "2. Second", "3. Third"} {
		if !strings.Contains(result, item) {
			t.Errorf("expected %q in:\n%s", item, result)
		}
	}
}

func TestConvertLinks(t *testing.T) {
	h := `<a href="https://example.com">Example</a>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "[Example](https://example.com)") {
		t.Errorf("expected markdown link in:\n%s", result)
	}
}

func TestConvertBoldItalic(t *testing.T) {
	h := `<strong>bold text</strong> and <em>italic text</em>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "**bold text**") {
		t.Errorf("expected **bold text** in:\n%s", result)
	}
	if !strings.Contains(result, "*italic text*") {
		t.Errorf("expected *italic text* in:\n%s", result)
	}
}

func TestConvertTable(t *testing.T) {
	h := `<table>
		<thead><tr><th>Name</th><th>Age</th></tr></thead>
		<tbody><tr><td>Alice</td><td>30</td></tr></tbody>
	</table>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "| Name | Age |") {
		t.Errorf("expected header row in:\n%s", result)
	}
	if !strings.Contains(result, "| --- | --- |") {
		t.Errorf("expected separator row in:\n%s", result)
	}
	if !strings.Contains(result, "| Alice | 30 |") {
		t.Errorf("expected data row in:\n%s", result)
	}
}

func TestConvertBlockquote(t *testing.T) {
	h := `<blockquote><p>This is a quote.</p></blockquote>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "> This is a quote.") {
		t.Errorf("expected > prefixed blockquote in:\n%s", result)
	}
}

func TestConvertHorizontalRule(t *testing.T) {
	h := `<p>Before</p><hr><p>After</p>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "---") {
		t.Errorf("expected --- horizontal rule in:\n%s", result)
	}
}

func TestConvertImage(t *testing.T) {
	h := `<img src="photo.png" alt="A photo">`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "![A photo](photo.png)") {
		t.Errorf("expected ![A photo](photo.png) in:\n%s", result)
	}
}

func TestConvertDefinitionList(t *testing.T) {
	h := `<dl><dt>Term</dt><dd>Definition of term.</dd></dl>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "**Term**") {
		t.Errorf("expected **Term** in:\n%s", result)
	}
	if !strings.Contains(result, "Definition of term.") {
		t.Errorf("expected definition text in:\n%s", result)
	}
}

func TestConvertNestedList(t *testing.T) {
	h := `<ul>
		<li>Parent
			<ul>
				<li>Child</li>
			</ul>
		</li>
	</ul>`
	doc := parseHTML(h)
	result := convertToMarkdown(doc)
	if !strings.Contains(result, "- Parent") {
		t.Errorf("expected parent list item in:\n%s", result)
	}
	// Child should be indented with 2 spaces
	if !strings.Contains(result, "  - Child") {
		t.Errorf("expected indented child list item '  - Child' in:\n%s", result)
	}
}
