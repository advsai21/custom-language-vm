package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ============================================================================
// 1. FRONTEND: AST TYPES, LEXER, & PARSER
// ============================================================================
type Symbol string
type Number float64
type List []any

func Tokenize(src string) []string {
	src = strings.ReplaceAll(src, "(", " ( ")
	src = strings.ReplaceAll(src, ")", " ) ")
	return strings.Fields(src)
}

func Parse(tokens []string) (any, []string, error) {
	if len(tokens) == 0 { return nil, nil, errors.New("unexpected end of file") }
	token := tokens[0]
	tokens = tokens[1:]

	if token == "(" {
		var list List
		for len(tokens) > 0 && tokens[0] != ")" {
			expr, remaining, err := Parse(tokens)
			if err != nil { return nil, nil, err }
			list = append(list, expr)
			tokens = remaining
		}
		if len(tokens) == 0 { return nil, nil, errors.New("missing closing parenthesis") }
		return list, tokens[1:], nil
	}
	if num, err := strconv.ParseFloat(token, 64); err == nil {
		return Number(num), tokens, nil
	}
	return Symbol(token), tokens, nil
}

// Extract multiple independent sequential code expressions out of a script file
func ParseMultiple(tokens []string) ([]any, error) {
	var expressions []any
	remaining := tokens
	for len(remaining) > 0 {
		expr, rem, err := Parse(remaining)
		if err != nil { return nil, err }
		expressions = append(expressions, expr)
		remaining = rem
	}
	return expressions, nil
}

// ============================================================================
// 2. THE MACHINE OPCODES
// ============================================================================
const (
	OpConstant uint8 = iota
	OpAdd
	OpSubtract
	OpMultiply
	OpGetLocal
	OpSetLocal
	OpCall
	OpReturn
	OpHalt
)

// ============================================================================
// 3. THE COMPILER CONTEXT STRUCTURES
// ============================================================================
type CompiledFunction struct {
	Instructions []uint8
	Constants    []float64
	NumLocals    int
	NumArgs      int
}

type Frame struct {
	cl        *CompiledFunction
	ip        int 
	baseFrame int 
}

type LocalSymbol struct {
	Name  Symbol
	Index int
}

type Compiler struct {
	instructions []uint8
	constants    []float64
	locals       []LocalSymbol 
}

func NewCompiler() *Compiler {
	return &Compiler{
		instructions: make([]uint8, 0),
		constants:    make([]float64, 0),
		locals:       make([]LocalSymbol, 0),
	}
}

func (c *Compiler) addConstant(val float64) uint8 {
	c.constants = append(c.constants, val)
	return uint8(len(c.constants) - 1)
}

func (c *Compiler) Compile(node any) error {
	switch v := node.(type) {
	case Number:
		idx := c.addConstant(float64(v))
		c.instructions = append(c.instructions, OpConstant, idx)

	case Symbol:
		for i := len(c.locals) - 1; i >= 0; i-- {
			if c.locals[i].Name == v {
				c.instructions = append(c.instructions, OpGetLocal, uint8(c.locals[i].Index))
				return nil
			}
		}
		return fmt.Errorf("compiler error: undefined local symbol '%s'", v)

	case List:
		if len(v) == 0 { return errors.New("empty list") }
		
		// Immediately Intercept Compound Execution Lists: ((lambda (x) ...) 7)
		if innerList, ok := v[0].(List); ok {
			if len(innerList) > 0 {
				if opSym, ok := innerList[0].(Symbol); ok && opSym == "lambda" {
					for i := 1; i < len(v); i++ {
						if err := c.Compile(v[i]); err != nil { return err }
					}
					if err := c.Compile(innerList); err != nil { return err }
					c.instructions = append(c.instructions, OpCall, uint8(len(v)-1))
					return nil
				}
			}
		}

		operator, ok := v[0].(Symbol)
		if !ok { return errors.New("operator must be a symbol") }

		if operator == "lambda" {
			paramsList, ok := v[1].(List)
			if !ok { return errors.New("lambda mapping expectations error") }

			fnCompiler := NewCompiler()
			for _, p := range paramsList {
				fnCompiler.locals = append(fnCompiler.locals, LocalSymbol{
					Name:  p.(Symbol),
					Index: len(fnCompiler.locals),
				})
			}

			err := fnCompiler.Compile(v[2])
			if err != nil { return err }
			fnCompiler.instructions = append(fnCompiler.instructions, OpReturn)

			compiledFn := &CompiledFunction{
				Instructions: fnCompiler.instructions,
				Constants:    fnCompiler.constants,
				NumLocals:    len(fnCompiler.locals),
				NumArgs:      len(paramsList),
			}

			c.constants = append(c.constants, 0.0) 
			constIdx := uint8(len(c.constants) - 1)
			
			c.instructions = append(c.instructions, OpConstant, constIdx)
			globalFnRegistry[constIdx] = compiledFn
			return nil
		}

		for i := 1; i < len(v); i++ {
			if err := c.Compile(v[i]); err != nil { return err }
		}

		switch operator {
		case "+":  c.instructions = append(c.instructions, OpAdd)
		case "-":  c.instructions = append(c.instructions, OpSubtract)
		case "*":  c.instructions = append(c.instructions, OpMultiply)
		default:   return fmt.Errorf("compiler error: unknown operator '%s'", operator)
		}
	}
	return nil
}

var globalFnRegistry = make(map[uint8]*CompiledFunction)

// ============================================================================
// 4. THE CALL-FRAME VIRTUAL MACHINE
// ============================================================================
type VM struct {
	frames    [64]*Frame 
	framesIdx int        
	stack     [256]any   
	sp        int        
}

func NewVM() *VM { return &VM{framesIdx: 0, sp: 0} }

func (vm *VM) push(val any) { vm.stack[vm.sp] = val; vm.sp++ }
func (vm *VM) pop() any { 
	if vm.sp <= 0 { return 0.0 }
	vm.sp--
	return vm.stack[vm.sp] 
}

func (vm *VM) Run(mainFn *CompiledFunction) any {
	vm.frames[vm.framesIdx] = &Frame{cl: mainFn, ip: 0, baseFrame: 0}
	vm.framesIdx++

	for vm.framesIdx > 0 {
		frame := vm.frames[vm.framesIdx-1]
		if frame.ip >= len(frame.cl.Instructions) {
			vm.framesIdx--
			continue
		}

		op := frame.cl.Instructions[frame.ip]
		frame.ip++

		switch op {
		case OpConstant:
			idx := frame.cl.Instructions[frame.ip]; frame.ip++
			if fn, exists := globalFnRegistry[idx]; exists {
				vm.push(fn)
			} else {
				vm.push(frame.cl.Constants[idx])
			}
		case OpAdd:
			b := vm.pop().(float64); a := vm.pop().(float64); vm.push(a + b)
		case OpSubtract:
			b := vm.pop().(float64); a := vm.pop().(float64); vm.push(a - b)
		case OpMultiply:
			b := vm.pop().(float64); a := vm.pop().(float64); vm.push(a * b)
		case OpGetLocal:
			offset := int(frame.cl.Instructions[frame.ip]); frame.ip++
			vm.push(vm.stack[frame.baseFrame+offset])
		case OpCall:
			numArgs := int(frame.cl.Instructions[frame.ip]); frame.ip++
			fnTarget := vm.pop().(*CompiledFunction)
			newFrameBase := vm.sp - numArgs
			vm.frames[vm.framesIdx] = &Frame{cl: fnTarget, ip: 0, baseFrame: newFrameBase}
			vm.framesIdx++
		case OpReturn:
			retVal := vm.pop() 
			vm.framesIdx--
			deadFrame := vm.frames[vm.framesIdx]
			vm.sp = deadFrame.baseFrame
			vm.push(retVal)
		case OpHalt:
			if vm.sp > 0 { return vm.pop() }
			return nil
		}
	}
	if vm.sp > 0 { return vm.pop() }
	return nil
}

// ============================================================================
// 5. THE RUNTIME ROUTING LOGIC (REPL OR FILE EXECUTOR)
// ============================================================================
func main() {
	// IF AN ARGUMENT IS PASSED: Read and execute the script file directly!
	if len(os.Args) > 1 {
		filename := os.Args[1]
		content, err := os.ReadFile(filename)
		if err != nil {
			fmt.Printf("❌ File System Error: Could not read script target '%s'\n", filename)
			return
		}

		tokens := Tokenize(string(content))
		expressions, err := ParseMultiple(tokens)
		if err != nil {
			fmt.Printf("❌ Script Parser Error: %v\n", err)
			return
		}

		compiler := NewCompiler()
		for _, expr := range expressions {
			if err := compiler.Compile(expr); err != nil {
				fmt.Printf("❌ Script Compilation Failure: %v\n", err)
				return
			}
		}
		compiler.instructions = append(compiler.instructions, OpHalt)

		mainFn := &CompiledFunction{
			Instructions: compiler.instructions,
			Constants:    compiler.constants,
		}

		vm := NewVM()
		result := vm.Run(mainFn)
		fmt.Printf("Script Output: %v\n", result)
		return
	}

	// FALLBACK ROUTE: Fire up the live interactive REPL interface
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("==================================================")
	fmt.Println("🚀 HIGH-PERFORMANCE BYTECODE VM REPL TERMINAL ONLINE.")
	fmt.Println("==================================================")

	for {
		fmt.Print("vm-lisp> ")
		if !scanner.Scan() { break }
		input := strings.TrimSpace(scanner.Text())
		if input == "" { continue }
		if input == "exit" { break }

		tokens := Tokenize(input)
		ast, _, err := Parse(tokens)
		if err != nil {
			fmt.Printf("❌ Parser Error: %v\n", err)
			continue
		}

		compiler := NewCompiler()
		if err := compiler.Compile(ast); err != nil {
			fmt.Printf("❌ Compiler Error: %v\n", err)
			continue
		}
		compiler.instructions = append(compiler.instructions, OpHalt)

		mainFn := &CompiledFunction{
			Instructions: compiler.instructions,
			Constants:    compiler.constants,
		}

		vm := NewVM()
		result := vm.Run(mainFn)
		fmt.Printf("=> %v\n", result)
	}
}