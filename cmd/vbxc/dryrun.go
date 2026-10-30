package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vbxc/ast"
	"vbxc/compiler"
	"vbxc/parser"
	"vbxc/plugin"
	"vbxc/stdlib"
)

type callWarning struct {
	name       string
	context    string
	suggestion string
}

// dryRun parses and compiles a source file without evaluating any statements.
// Plugins are registered so calls provided by #use modules are recognized.
func dryRun(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var modules []string
	stmts, err := parser.ParseSource(strings.TrimPrefix(string(data), "\uFEFF"), parser.Options{
		File:           filepath.Base(path),
		BaseDir:        filepath.Dir(path),
		StrictIncludes: true,
		ExtractUse:     blankDirectives,
		OnModules:      func(m []string) { modules = append(modules, m...) },
	})
	if err != nil {
		return 0, err
	}
	if len(stmts) == 0 {
		return 0, fmt.Errorf("keine VBX-Anweisungen gefunden")
	}
	if err := plugin.LoadModules(modules); err != nil {
		return 0, err
	}
	if _, err := compiler.Compile(filepath.Base(path), stmts); err != nil {
		return 0, err
	}
	warnings := checkUnknownCalls(stmts)
	for _, w := range warnings {
		msg := fmt.Sprintf("[WARNUNG] (%s) Unbekannter Aufruf: '%s'", w.context, w.name)
		if w.suggestion != "" {
			msg += fmt.Sprintf(" - meinten Sie '%s'?", w.suggestion)
		}
		fmt.Println(msg)
	}
	return len(warnings), nil
}

func checkUnknownCalls(stmts []ast.Stmt) []callWarning {
	known := make(map[string]bool)
	for name := range stdlib.Builtins() {
		known[strings.ToLower(name)] = true
	}
	topVars := make(map[string]bool)
	procNames := make(map[string]bool)
	collectVars(stmts, topVars, procNames)
	for name := range procNames {
		known[name] = true
	}
	c := callChecker{known: known}
	c.walk(stmts, "Top-Level", topVars)
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.SubNode:
			vars := cloneVars(topVars)
			for _, p := range n.Params {
				vars[strings.ToLower(p.Name)] = true
			}
			c.walk(n.Body, "Sub "+n.Name, vars)
		case *ast.FuncNode:
			vars := cloneVars(topVars)
			for _, p := range n.Params {
				vars[strings.ToLower(p.Name)] = true
			}
			c.walk(n.Body, "Function "+n.Name, vars)
		}
	}
	return c.warnings
}

type callChecker struct {
	known    map[string]bool
	warnings []callWarning
}

func cloneVars(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for name := range src {
		dst[name] = true
	}
	return dst
}

func collectVars(stmts []ast.Stmt, vars, procs map[string]bool) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.AssignNode:
			if n.IsDeclaration {
				vars[strings.ToLower(n.Name)] = true
			}
		case *ast.PublicNode:
			vars[strings.ToLower(n.Name)] = true
		case *ast.DimArrayNode:
			vars[strings.ToLower(n.Name)] = true
		case *ast.PublicArrayNode:
			vars[strings.ToLower(n.Name)] = true
		case *ast.ForNode:
			vars[strings.ToLower(n.VarName)] = true
			collectVars(n.Body, vars, procs)
		case *ast.ForEachNode:
			vars[strings.ToLower(n.KeyVar)] = true
			if n.ValVar != "" {
				vars[strings.ToLower(n.ValVar)] = true
			}
			collectVars(n.Body, vars, procs)
		case *ast.IfNode:
			for _, b := range n.Branches {
				collectVars(b.Body, vars, procs)
			}
			collectVars(n.Else, vars, procs)
		case *ast.SelectNode:
			for _, b := range n.Cases {
				collectVars(b.Body, vars, procs)
			}
			collectVars(n.Default, vars, procs)
		case *ast.WhileNode:
			collectVars(n.Body, vars, procs)
		case *ast.DoLoopNode:
			collectVars(n.Body, vars, procs)
		case *ast.TryNode:
			if n.CatchVarName != "" {
				vars[strings.ToLower(n.CatchVarName)] = true
			}
			collectVars(n.TryBody, vars, procs)
			collectVars(n.CatchBody, vars, procs)
			collectVars(n.FinallyBody, vars, procs)
		case *ast.SubNode:
			procs[strings.ToLower(n.Name)] = true
		case *ast.FuncNode:
			procs[strings.ToLower(n.Name)] = true
		case *ast.MultiStmtNode:
			collectVars(n.Stmts, vars, procs)
		}
	}
}

func (c *callChecker) warn(name, context string) {
	if name == "" || c.known[strings.ToLower(name)] {
		return
	}
	c.warnings = append(c.warnings, callWarning{name: name, context: context, suggestion: closestCallSuggestion(name, c.known)})
}

func (c *callChecker) walk(stmts []ast.Stmt, context string, vars map[string]bool) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.AssignNode:
			c.expr(n.Value, context, vars)
		case *ast.CompoundAssignNode:
			c.expr(n.Left, context, vars)
			c.expr(n.Right, context, vars)
		case *ast.ArrayAssignNode:
			c.expr(n.Index, context, vars)
			c.expr(n.Index2, context, vars)
			c.expr(n.Value, context, vars)
		case *ast.DimArrayNode:
			c.expr(n.Size1, context, vars)
			c.expr(n.Size2, context, vars)
		case *ast.PublicNode:
			c.expr(n.Value, context, vars)
		case *ast.PublicArrayNode:
			c.expr(n.Size1, context, vars)
			c.expr(n.Size2, context, vars)
		case *ast.PrintNode:
			c.expr(n.Value, context, vars)
			c.expr(n.Color, context, vars)
		case *ast.IfNode:
			for _, b := range n.Branches {
				c.expr(b.Cond, context, vars)
				c.walk(b.Body, context, vars)
			}
			c.walk(n.Else, context, vars)
		case *ast.SelectNode:
			c.expr(n.Expression, context, vars)
			for _, b := range n.Cases {
				for _, cond := range b.Conditions {
					c.expr(cond, context, vars)
				}
				c.walk(b.Body, context, vars)
			}
			c.walk(n.Default, context, vars)
		case *ast.ForNode:
			c.expr(n.Start, context, vars)
			c.expr(n.End, context, vars)
			c.walk(n.Body, context, vars)
		case *ast.ForEachNode:
			c.expr(n.Iterable, context, vars)
			c.walk(n.Body, context, vars)
		case *ast.WhileNode:
			c.expr(n.Condition, context, vars)
			c.walk(n.Body, context, vars)
		case *ast.DoLoopNode:
			c.expr(n.Condition, context, vars)
			c.walk(n.Body, context, vars)
		case *ast.SubNode, *ast.FuncNode:
			// Procedures are checked separately with their own local scope.
		case *ast.CallNode:
			c.checkCall(n.Name, context, vars)
			for _, arg := range n.Args {
				c.expr(arg, context, vars)
			}
		case *ast.ReturnNode:
			c.expr(n.Value, context, vars)
		case *ast.TryNode:
			c.walk(n.TryBody, context, vars)
			c.walk(n.CatchBody, context, vars)
			c.walk(n.FinallyBody, context, vars)
		case *ast.MultiStmtNode:
			c.walk(n.Stmts, context, vars)
		}
	}
}

func (c *callChecker) checkCall(name, context string, vars map[string]bool) {
	lower := strings.ToLower(name)
	if c.known[lower] || vars[lower] {
		return
	}
	if i := strings.IndexByte(lower, '.'); i > 0 && vars[lower[:i]] {
		return
	}
	c.warn(name, context)
}

func (c *callChecker) expr(e ast.Expr, context string, vars map[string]bool) {
	if e == nil {
		return
	}
	switch n := e.(type) {
	case *ast.CallExprNode:
		c.checkCall(n.Name, context, vars)
		for _, arg := range n.Args {
			c.expr(arg, context, vars)
		}
	case *ast.VarNode:
		if strings.Contains(n.Name, ".") {
			if _, ok := builtinForQualifiedName(n.Name); ok {
				c.warnings = append(c.warnings, callWarning{name: n.Name, context: context, suggestion: n.Name + "()"})
			}
		}
	case *ast.UnaryOpNode:
		c.expr(n.Right, context, vars)
	case *ast.BinOpNode:
		c.expr(n.Left, context, vars)
		c.expr(n.Right, context, vars)
	case *ast.MapIndexNode:
		c.expr(n.Base, context, vars)
		c.expr(n.Key, context, vars)
	case *ast.ArrayLiteralNode:
		for _, item := range n.Elements {
			c.expr(item, context, vars)
		}
	case *ast.RangeNode:
		c.expr(n.Low, context, vars)
		c.expr(n.High, context, vars)
	case *ast.IsNode:
		c.expr(n.Value, context, vars)
	}
}

func builtinForQualifiedName(name string) (stdlib.BuiltinInfo, bool) {
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		return stdlib.BuiltinInfo{}, false
	}
	return stdlib.FindByModuleAndName(parts[0], parts[1])
}

func closestCallSuggestion(name string, known map[string]bool) string {
	want := strings.ToLower(name)
	best, bestDistance := "", 4
	names := make([]string, 0, len(known))
	for candidate := range known {
		names = append(names, candidate)
	}
	sort.Strings(names)
	for _, candidate := range names {
		if candidate == want {
			continue
		}
		d := editDistance(want, candidate)
		if d < bestDistance {
			best, bestDistance = candidate, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			cur[j] = min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
