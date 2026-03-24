import type { Component, JSX } from "solid-js";

type ActionButtonProps = {
  tip: string;
  icon: JSX.Element;
  onClick: () => void;
  opacity?: string;
};

export const ActionButton: Component<ActionButtonProps> = (props) => (
  <div class="tooltip tooltip-left" data-tip={props.tip}>
    <button
      type="button"
      class={`btn btn-ghost btn-xs p-0.5 h-auto min-h-0 ${props.opacity ?? "opacity-60"} hover:opacity-100`}
      onClick={(e) => {
        e.stopPropagation();
        props.onClick();
      }}
    >
      {props.icon}
    </button>
  </div>
);
