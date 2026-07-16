// 示例扩展：安全检查
// 这个扩展会拦截危险的 bash 命令

import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  // 拦截工具调用
  pi.on("tool_call", async (event, ctx) => {
    // 检查 bash 命令
    if (event.toolName === "bash") {
      const command = event.input.command || "";
      
      // 危险命令列表
      const dangerousPatterns = [
        /rm\s+-rf\s+\//,
        /sudo/,
        /chmod\s+777/,
        /curl.*\|\s*sh/,
        /wget.*\|\s*sh/,
        /:\(\)\{.*\}:/,  // fork bomb
      ];
      
      for (const pattern of dangerousPatterns) {
        if (pattern.test(command)) {
          // 请求用户确认
          const confirmed = await ctx.ui.confirm(
            "⚠️ 危险命令检测",
            `检测到可能危险的命令：\n\n${command}\n\n是否允许执行？`
          );
          
          if (!confirmed) {
            return { block: true, reason: "用户拒绝执行危险命令" };
          }
        }
      }
    }
    
    // 检查文件写入
    if (event.toolName === "write" || event.toolName === "edit") {
      const path = event.input.path || "";
      
      // 保护敏感文件
      const protectedPaths = [
        /\.env$/,
        /\.git\//,
        /node_modules\//,
        /\.ssh\//,
      ];
      
      for (const pattern of protectedPaths) {
        if (pattern.test(path)) {
          const confirmed = await ctx.ui.confirm(
            "⚠️ 敏感文件写入",
            `尝试写入受保护的文件：\n\n${path}\n\n是否允许？`
          );
          
          if (!confirmed) {
            return { block: true, reason: "用户拒绝写入敏感文件" };
          }
        }
      }
    }
  });
  
  // 会话开始时的通知
  pi.on("session_start", async (_event, ctx) => {
    ctx.ui.notify("🛡️ 安全扩展已加载", "info");
  });
  
  // 注册命令
  pi.registerCommand("security-status", {
    description: "显示安全扩展状态",
    handler: async (_args, ctx) => {
      ctx.ui.notify("安全扩展运行正常", "info");
    },
  });
}
