import { tool } from "@opencode-ai/plugin"

export const AliceDelegate = async ({ $, directory }) => ({
  tool: {
    codex: tool({
      description:
        "Delegate a task to OpenAI Codex CLI agent. " +
        "Use for code editing, sandbox execution, or repository-wide changes. " +
        "Prefer codex over claude when the task involves writing, editing, or searching code.",
      args: {
        prompt: tool.schema.string().describe("Task description for Codex"),
      },
      async execute(args, context) {
        try {
          const wsDir = context.directory || directory
          return await $`alice delegate --provider codex --workspace-dir ${wsDir} --prompt ${args.prompt}`.text()
        } catch (e) {
          return "Codex delegation error: " + (e.message || String(e))
        }
      },
    }),
    claude: tool({
      description:
        "Delegate a task to Anthropic Claude CLI agent. " +
        "Use for analysis, code review, explanation, documentation, or debugging. " +
        "Prefer claude over codex for tasks that require deep reasoning or explanation rather than code generation.",
      args: {
        prompt: tool.schema.string().describe("Task description for Claude"),
      },
      async execute(args, context) {
        try {
          const wsDir = context.directory || directory
          return await $`alice delegate --provider claude --workspace-dir ${wsDir} --prompt ${args.prompt}`.text()
        } catch (e) {
          return "Claude delegation error: " + (e.message || String(e))
        }
      },
    }),
  },
})
