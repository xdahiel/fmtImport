package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage transfer [DIR]")
		return
	}

	log.SetFlags(log.LstdFlags)
	err := filepath.Walk(os.Args[1], func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			log.Println(err)
			return err
		}

		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		// 创建一个文件集合
		fset := token.NewFileSet()

		// 解析源代码文件
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			fmt.Println("Error parsing file:", err)
			return err
		}

		pkgs := make([]string, 0)
		names := make(map[string]string)

		// 遍历AST树，查找import声明
		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.IMPORT {
				continue
			}

			// 遍历import声明列表
			for _, spec := range genDecl.Specs {
				importSpec, ok := spec.(*ast.ImportSpec)
				if !ok {
					continue
				}

				// 获取import路径
				importPath := importSpec.Path.Value

				// 获取别名
				alias := ""
				if importSpec.Name != nil {
					alias = importSpec.Name.Name
				}
				names[importPath] = alias
				pkgs = append(pkgs, importPath)
			}
		}

		// 删除import声明
		for i, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.IMPORT {
				continue
			}

			// 删除import声明
			f.Decls = append(f.Decls[:i], f.Decls[i+1:]...)
			break
		}

		if len(pkgs) == 0 {
			return err
		}

		// 根据包名排序
		//fmt.Println(pkgs)
		pkgs = sortPkg(pkgs)
		//fmt.Println(pkgs)

		// 重新编写import部分
		newImports := make([]*ast.ImportSpec, 0, len(names))
		for _, pkg := range pkgs {
			spec := &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: pkg,
				},
			}

			if name, ok := names[pkg]; ok {
				spec.Name = &ast.Ident{Name: name}
			} else {
				spec.Name = nil
			}
			newImports = append(newImports, spec)
		}

		// 添加新的import声明
		f.Decls = append([]ast.Decl{&ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: []ast.Spec{},
		}}, f.Decls...)

		genDecl, _ := f.Decls[0].(*ast.GenDecl)
		for _, ipt := range newImports {
			genDecl.Specs = append(genDecl.Specs, ipt)
		}

		// 格式化AST并输出新的源代码
		var buf strings.Builder
		err = format.Node(&buf, fset, f)
		if err != nil {
			fmt.Println("Error formatting code:", err)
			return err
		}

		fl, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}

		_, err = fl.WriteString(buf.String())
		if err != nil {
			return err
		}
		cmd := exec.Command("go", "fmt", path)
		fmt.Println(cmd.String())
		return cmd.Run()
	})

	if err != nil {
		panic(err)
	}
}

func sortPkg(pkgs []string) []string {
	mp := [3][]string{}
	for i := 0; i < 3; i++ {
		mp[i] = make([]string, 0)
	}

	for _, pkg := range pkgs {
		if pkg == "" || pkg == "\n" {
			continue
		}
		//fmt.Println(len(pkg), pkg)
		d := pkgDegree(pkg)
		mp[d] = append(mp[d], pkg)
	}

	for _, arr := range mp {
		sort.Slice(arr, func(i, j int) bool {
			return arr[i] < arr[j]
		})
	}

	res := make([]string, 0, len(pkgs))
	for i := 0; i < 3; i++ {
		res = append(res, mp[i]...)
		//fmt.Println(mp[i], len(mp[i]))
		if i == 0 {
			if len(mp[1]) > 0 || len(mp[2]) > 0 {
				res = append(res, "")
			}
		} else if i == 1 {
			if len(mp[2]) > 0 {
				res = append(res, "")
			}
		}
	}

	return res
}

func pkgDegree(s string) int {
	if !strings.Contains(s, ".") {
		return 0
	}

	if strings.Contains(s, "git.ucloudadmin.com") {
		return 1
	}
	return 2
}
