declare module "@backend" {
  import type { TmpUsage, Workspace, WorkspaceConfig } from "@/types/types";

  export const AppService: {
    GetVersion(): Promise<string>;
    InstallUpdate(version: string): Promise<void>;
  };

  export const MonitorService: {
    GetWorkspaces(): Promise<Workspace[]>;
    GetTmpUsage(): Promise<TmpUsage>;
    NukeTmpFiles(): Promise<void>;
    RefreshNow(): Promise<void>;
  };

  export const WorkspaceService: {
    GetWorkspaces(): Promise<Workspace[]>;
    AddWorkspace(repoPath: string): Promise<string>;
    RemoveWorkspace(name: string): Promise<void>;
    CreateWorktree(workspaceName: string, worktreeName: string): Promise<void>;
    RemoveWorktree(workspaceName: string, worktreeName: string): Promise<void>;
    ForceRemoveWorktree(
      workspaceName: string,
      worktreeName: string,
    ): Promise<void>;
    CancelTask(workspaceName: string, worktreeName: string): Promise<void>;
    RetrySetup(workspaceName: string, worktreeName: string): Promise<void>;
    RetryArchive(workspaceName: string, worktreeName: string): Promise<void>;
    GetWorkspaceConfig(name: string): Promise<WorkspaceConfig>;
    UpdateWorkspaceConfig(name: string, config: WorkspaceConfig): Promise<void>;
    OpenFolderDialog(): Promise<string>;
  };

  export type EditorBounds = {
    x: number;
    y: number;
    width: number;
    height: number;
  };

  export const SnapService: {
    SetEnabled(enabled: boolean): Promise<void>;
    SnapNow(): Promise<void>;
    GetSnapSide(): Promise<string>;
    GetEditorBounds(): Promise<EditorBounds>;
  };

  export const TrayService: {
    SetEnabled(enabled: boolean): Promise<void>;
  };

  export const EditorService: {
    IsValidApp(name: string): Promise<boolean>;
    FocusEditor(worktreePath: string, editorApp: string): Promise<void>;
    PositionWindow(
      appName: string,
      x: number,
      y: number,
      width: number,
      height: number,
    ): Promise<void>;
  };

  export const SoundService: {
    GetSounds(): Promise<string[]>;
    SetPreferences(mode: string, sound: string): Promise<void>;
    PlayPreview(name: string): Promise<void>;
  };
}
