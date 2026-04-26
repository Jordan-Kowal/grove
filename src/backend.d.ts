declare module "@backend" {
  import type { BranchInfo, Workspace, WorkspaceConfig } from "@/types/types";

  export const AppService: {
    GetVersion(): Promise<string>;
    InstallUpdate(version: string): Promise<void>;
    IsAccessibilityTrusted(): Promise<boolean>;
  };

  export const MonitorService: {
    Snapshot(): Promise<Workspace[]>;
    RefreshNow(): Promise<void>;
    DismissDone(path: string): Promise<void>;
    SetDoneDuration(minutes: number): Promise<void>;
    SetEditorApp(appName: string): Promise<void>;
    SetEditorTrackingEnabled(enabled: boolean): Promise<void>;
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
    ListBranches(workspaceName: string): Promise<BranchInfo[]>;
    RebaseWorktree(
      workspaceName: string,
      worktreeName: string,
      targetBranch: string,
    ): Promise<void>;
    CheckoutBranch(
      workspaceName: string,
      worktreeName: string,
      branch: string,
    ): Promise<void>;
    NewBranchOnWorktree(
      workspaceName: string,
      worktreeName: string,
      branchName: string,
    ): Promise<void>;
    OpenFolderDialog(): Promise<string>;
    SyncMainCheckout(workspaceName: string): Promise<void>;
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
    GetEditorBounds(widthPercent: number): Promise<EditorBounds>;
    OpenAccessibilitySettings(): Promise<void>;
  };

  export const TrayService: {
    SetEnabled(enabled: boolean): Promise<void>;
  };

  export const EditorService: {
    IsValidApp(name: string): Promise<boolean>;
    FocusEditor(worktreePath: string, editorApp: string): Promise<void>;
    CloseEditorWindow(worktreePath: string, editorApp: string): Promise<void>;
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
    SetPreferences(mode: string, sound: string): Promise<void>; // rejects on invalid mode/sound
    PlayPreview(name: string): Promise<void>;
  };
}
