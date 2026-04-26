import { EditorService, MonitorService, SnapService } from "@backend";
import type { Settings } from "@/contexts";
import type { Workspace } from "@/types/types";

type Params = {
  workspaces: Workspace[];
  settings: () => Settings;
};

export const useEditorActions = ({ workspaces, settings }: Params) => {
  const focusEditor = async (worktreePath: string) => {
    MonitorService.DismissDone(worktreePath);
    const editorApp = settings().editorApp;
    try {
      await EditorService.FocusEditor(worktreePath, editorApp);
      const side = await SnapService.GetSnapSide();
      if (side) {
        const bounds = await SnapService.GetEditorBounds(
          settings().ideDockWidthPercent,
        );
        if (bounds.width > 0) {
          await EditorService.PositionWindow(
            editorApp,
            bounds.x,
            bounds.y,
            bounds.width,
            bounds.height,
          );
        }
      }
    } catch (e) {
      console.error("[grove] focusEditor failed:", e);
    }
  };

  const closeEditor = async (worktreePath: string) => {
    const editorApp = settings().editorApp;
    try {
      await EditorService.CloseEditorWindow(worktreePath, editorApp);
      MonitorService.RefreshNow();
    } catch (e) {
      console.error("[grove] closeEditor failed:", e);
    }
  };

  const closeAllEditors = async (workspaceName: string) => {
    const ws = workspaces.find((w) => w.name === workspaceName);
    if (!ws) return;
    const editorApp = settings().editorApp;
    const paths: string[] = [];
    if (ws.mainWorktree.editorOpen) paths.push(ws.mainWorktree.path);
    for (const wt of ws.worktrees ?? []) {
      if (wt.editorOpen) paths.push(wt.path);
    }
    try {
      for (const p of paths) {
        await EditorService.CloseEditorWindow(p, editorApp);
      }
      MonitorService.RefreshNow();
    } catch (e) {
      console.error("[grove] closeAllEditors failed:", e);
    }
  };

  return { focusEditor, closeEditor, closeAllEditors };
};
