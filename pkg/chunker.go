package pkg

import (
	"os"
	"path/filepath"
	"strings"

	"deepseek_wiki/model"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

type Chunker struct{}

func NewChunker() *Chunker {
	return &Chunker{}
}

func (c *Chunker) ChunkFile(filePath, repoPath string) ([]model.CodeChunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(repoPath, filePath)
	ext := filepath.Ext(filePath)
	language := detectLanguage(ext)

	lang := c.getLanguage(ext)
	if lang == nil {
		return c.chunkGenericFile(content, relPath, language), nil
	}

	return c.chunkWithTreeSitter(content, relPath, language, lang)
}

func (c *Chunker) getLanguage(ext string) *gotreesitter.Language {
	switch ext {
	case ".go":
		return grammars.GoLanguage()
	case ".py":
		return grammars.PythonLanguage()
	case ".js":
		return grammars.JavascriptLanguage()
	case ".ts":
		return grammars.TypescriptLanguage()
	case ".java":
		return grammars.JavaLanguage()
	case ".cpp", ".cxx", ".cc":
		return grammars.CppLanguage()
	case ".c":
		return grammars.CLanguage()
	case ".rs":
		return grammars.RustLanguage()
	case ".rb":
		return grammars.RubyLanguage()
	case ".php":
		return grammars.PhpLanguage()
	default:
		return nil
	}
}

func (c *Chunker) chunkWithTreeSitter(content []byte, relPath, language string, lang *gotreesitter.Language) ([]model.CodeChunk, error) {
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil || tree == nil {
		return c.chunkGenericFile(content, relPath, language), nil
	}

	root := tree.RootNode()
	if root == nil {
		return c.chunkGenericFile(content, relPath, language), nil
	}

	nodeTypes := c.getNodeTypes(language)
	chunks := c.extractChunks(root, content, relPath, language, lang, nodeTypes)

	if len(chunks) == 0 {
		return c.chunkGenericFile(content, relPath, language), nil
	}

	return chunks, nil
}

func (c *Chunker) getNodeTypes(language string) map[string]string {
	switch language {
	case "Go":
		return map[string]string{
			"function_declaration":  "function",
			"method_declaration":    "method",
			"interface_declaration": "interface",
			"struct_declaration":    "struct",
		}
	case "Python":
		return map[string]string{
			"function_definition": "function",
			"class_definition":    "class",
		}
	case "JavaScript", "TypeScript":
		return map[string]string{
			"function_declaration": "function",
			"method_definition":    "method",
			"class_declaration":    "class",
			"arrow_function":       "function",
		}
	case "Java":
		return map[string]string{
			"method_declaration":    "method",
			"class_declaration":     "class",
			"interface_declaration": "interface",
		}
	case "C", "C++":
		return map[string]string{
			"function_definition": "function",
			"class_specifier":     "class",
		}
	case "Rust":
		return map[string]string{
			"function_item": "function",
			"struct_item":   "struct",
			"trait_item":    "trait",
			"impl_item":     "impl",
		}
	case "Ruby":
		return map[string]string{
			"method": "method",
			"class":  "class",
			"module": "module",
		}
	case "PHP":
		return map[string]string{
			"function_definition": "function",
			"class_declaration":   "class",
			"method_declaration":  "method",
		}
	default:
		return map[string]string{}
	}
}

func (c *Chunker) extractChunks(root *gotreesitter.Node, content []byte, relPath, language string, lang *gotreesitter.Language, nodeTypeMap map[string]string) []model.CodeChunk {
	var chunks []model.CodeChunk

	c.walkNode(root, lang, func(node *gotreesitter.Node) bool {
		nodeType := node.Type(lang)

		if symbolType, ok := nodeTypeMap[nodeType]; ok {
			chunk := c.nodeToChunk(node, content, relPath, language, symbolType, lang)
			if chunk != nil {
				chunks = append(chunks, *chunk)
			}
		}

		if c.shouldSkipChildren(nodeType, language) {
			return false
		}

		return true
	})

	return chunks
}

func (c *Chunker) shouldSkipChildren(nodeType, language string) bool {
	skipTypes := map[string]bool{
		"comment":            true,
		"string":             true,
		"string_literal":     true,
		"import_declaration": true,
		"package_clause":     true,
	}
	return skipTypes[nodeType]
}

func (c *Chunker) walkNode(node *gotreesitter.Node, lang *gotreesitter.Language, fn func(*gotreesitter.Node) bool) {
	if node == nil {
		return
	}
	if !fn(node) {
		return
	}

	childCount := node.ChildCount()
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child != nil {
			c.walkNode(child, lang, fn)
		}
	}
}

func (c *Chunker) nodeToChunk(node *gotreesitter.Node, content []byte, relPath, language, symbolType string, lang *gotreesitter.Language) *model.CodeChunk {
	startByte := node.StartByte()
	endByte := node.EndByte()

	if int(endByte) > len(content) {
		return nil
	}

	chunkContent := string(content[startByte:endByte])
	startPoint := node.StartPoint()
	endPoint := node.EndPoint()

	symbolName := c.extractSymbolName(node, content, lang)

	return &model.CodeChunk{
		FilePath:   relPath,
		StartLine:  int(startPoint.Row) + 1,
		EndLine:    int(endPoint.Row) + 1,
		Language:   language,
		SymbolName: symbolName,
		SymbolType: symbolType,
		Content:    chunkContent,
	}
}

func (c *Chunker) extractSymbolName(node *gotreesitter.Node, content []byte, lang *gotreesitter.Language) string {
	childCount := node.ChildCount()
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := child.Type(lang)
		if !c.isNameNode(childType) {
			continue
		}

		start := child.StartByte()
		end := child.EndByte()
		if int(end) <= len(content) {
			name := string(content[start:end])
			if len(name) > 0 && len(name) < 100 {
				return name
			}
		}
	}
	return ""
}

func (c *Chunker) isNameNode(nodeType string) bool {
	nameTypes := map[string]bool{
		"identifier":          true,
		"type_identifier":     true,
		"name":                true,
		"property_identifier": true,
		"field_identifier":    true,
		"method_name":         true,
	}
	return nameTypes[nodeType]
}

func (c *Chunker) chunkGenericFile(content []byte, relPath, language string) []model.CodeChunk {
	lines := strings.Split(string(content), "\n")

	return []model.CodeChunk{
		{
			FilePath:   relPath,
			StartLine:  1,
			EndLine:    len(lines),
			Language:   language,
			SymbolType: "file",
			Content:    string(content),
		},
	}
}

func detectLanguage(ext string) string {
	langMap := map[string]string{
		".go":   "Go",
		".py":   "Python",
		".js":   "JavaScript",
		".ts":   "TypeScript",
		".java": "Java",
		".cpp":  "C++",
		".cxx":  "C++",
		".cc":   "C++",
		".c":    "C",
		".rs":   "Rust",
		".rb":   "Ruby",
		".php":  "PHP",
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "Unknown"
}
