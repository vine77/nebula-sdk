package generator

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/puppetlabs/nebula-sdk/pkg/container/def"
)

type dockerfile string

func (d dockerfile) Filename() string {
	if d == "base" {
		return "Dockerfile"
	}

	return fmt.Sprintf("Dockerfile.%s", d)
}

type imageTemplateImageData struct {
	Ref string
}

type imageTemplateData struct {
	Name       string
	SDKVersion string
	Images     map[string]*imageTemplateImageData
	Settings   map[string]interface{}
}

func generateImage(c *def.Container, imageName string, data *imageTemplateData, ref *def.FileRef) (*File, error) {
	image := c.Images[imageName]

	tpl, err := template.New(image.TemplateName).Funcs(sprig.TxtFuncMap()).Parse(image.TemplateData)
	if err != nil {
		return nil, &ImageTemplateParseError{ImageName: imageName, Cause: err}
	}

	// Set up our buffer.
	var buf bytes.Buffer
	buf.WriteString("# This file is automatically generated by the Nebula SDK. DO NOT EDIT.\n\n")

	if err := tpl.Execute(&buf, data); err != nil {
		return nil, &ImageTemplateExecutionError{ImageName: imageName, Cause: err}
	}

	content := buf.String()

	// Make sure that the rendered output is a valid Dockerfile.
	result, err := parser.Parse(strings.NewReader(content))
	if err != nil {
		return nil, &ImageTemplateFormatError{ImageName: imageName, Content: content, Cause: err}
	}

	s, _, err := instructions.Parse(result.AST)
	if err != nil {
		return nil, &ImageTemplateFormatError{ImageName: imageName, Content: content, Cause: err}
	} else if len(s) == 0 {
		return nil, &ImageTemplateNoStagesError{ImageName: imageName, Content: content}
	}

	// One of the reasons we had to actually parse the files as Dockerfiles is
	// to extract the escape character, which we need to inject multi-line
	// labels.
	content += fmt.Sprintf("\n%s", imageLabel("org.opencontainers.image.title", c.Title, result.EscapeToken))
	content += fmt.Sprintf("\n%s", imageLabel("org.opencontainers.image.description", c.Description, result.EscapeToken))
	content += fmt.Sprintf("\n%s", imageLabel("com.puppet.nebula.sdk.version", c.SDKVersion, result.EscapeToken))
	content += "\n"

	f := &File{
		Ref:     ref,
		Mode:    0644,
		Content: content,
	}
	return f, nil
}

func (g *Generator) generateImages() ([]*File, error) {
	data := &imageTemplateData{
		Name:       g.c.Name,
		SDKVersion: g.c.SDKVersion,
		Images:     make(map[string]*imageTemplateImageData, len(g.c.Images)),
		Settings:   make(map[string]interface{}, len(g.c.Settings)),
	}

	for imageName := range g.c.Images {
		data.Images[imageName] = &imageTemplateImageData{Ref: g.imageRef(imageName)}
	}

	for settingName, setting := range g.c.Settings {
		data.Settings[settingName] = setting.Value
	}

	var fs []*File
	for imageName := range g.c.Images {
		f, err := generateImage(g.c, imageName, data, g.base.Join(dockerfile(imageName).Filename()))
		if err != nil {
			return nil, err
		}

		fs = append(fs, f)
	}

	return fs, nil
}

func (g *Generator) relativeImageName(imageName string) string {
	name := g.c.Name

	if len(name) > 64 {
		name = name[:64]
	}

	if imageName != "base" {
		if len(imageName) > 64 {
			imageName = imageName[:64]
		}

		name += "-" + imageName
	}

	return name
}

func (g *Generator) absoluteImageName(imageName string) string {
	return path.Join(g.repoNameBase, g.relativeImageName(imageName))
}

func (g *Generator) imageRef(imageName string) string {
	return path.Join(IntermediateRepoNameBase, g.c.ID[:12], g.relativeImageName(imageName))
}

func newDockerfileQuoteEscaper(token string) *strings.Replacer {
	// Dockerfiles basically follow shell quoting rules:
	//
	// https://github.com/moby/buildkit/blob/9336f89e1fed39901a303996da63f4135c6216fe/frontend/dockerfile/shell/lex.go#L225-L235

	return strings.NewReplacer(
		token, token+token, // Escape the token itself
		`"`, token+`"`, // Escape double quotes
		`$`, token+`$`, // Escape dollar signs
		"\r\n", token+"\r\n", // Escape newlines
		"\n", token+"\n",
	)
}

var (
	// The two most common (only supported?) escape characters are backslash and
	// backtick.
	replacers = map[rune]*strings.Replacer{
		'\\': newDockerfileQuoteEscaper(`\`),
		'`':  newDockerfileQuoteEscaper("`"),
	}
)

func imageQuoteEscape(data string, token rune) string {
	replacer, found := replacers[token]
	if !found {
		replacer = newDockerfileQuoteEscaper(string(token))
	}

	return `"` + replacer.Replace(data) + `"`
}

func imageLabel(name, value string, escapeToken rune) string {
	return fmt.Sprintf(
		`LABEL %s=%s`,
		imageQuoteEscape(strings.TrimSpace(name), escapeToken),
		imageQuoteEscape(strings.TrimSpace(value), escapeToken),
	)
}
