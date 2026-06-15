# High-Performance Lisp Bytecode VM & Compiler

A custom, stack-based Virtual Machine and Ahead-of-Time Compiler built from scratch in Go. This engine transitions from a standard abstract syntax tree (AST) evaluation process to a fast, zero-string-allocation bytecode execution pipeline, mimicking the architectural layouts of modern execution runtimes like Python or the JVM.

## 🚀 Key Architectural Features
* **Recursive Lexer & Parser:** Tokenizes input character arrays into structured parenthetical Abstract Syntax Trees (ASTs).
* **The Compiler Pass:** Recursively flattens high-level expression clusters down into flat linear byte arrays (`[]uint8`).
* **Optimized Opcode Set:** Custom execution instruction bytes tracking operations like `OpConstant`, `OpAdd`, `OpMultiply`, and low-level pointer redirection commands.
* **Low-Level Call Stack:** Manages functional program flow via standalone `Call Frames` tracking execution offsets, arguments base registers, and independent tracking pointers.
* **Dual Execution Interface:** Implements a direct file script routing engine alongside a live interactive REPL console loop.

---

## 🛠️ The Pipeline Architecture
Instead of evaluating nodes recursively during runtime, the engine translates text down to optimized machine execution structures:

```text
[ Text Input Code ] ➔ ((lambda (x) (* x 10)) 7)
       │
       ▼
[ Lexer / Parser ]  ➔ AST Structure Nodes
       │
       ▼
[ Bytecode Compiler]➔ Generates Constant Pools & Instructions: [0 0 0 1 4 5 ... ]
       │
       ▼
[ Virtual Machine ] ➔ Spins Call Frames over the Data Stack at native CPU speeds
       │
       ▼
[ Result Return ]   ➔ => 70