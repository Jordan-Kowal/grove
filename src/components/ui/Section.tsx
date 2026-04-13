import type { Component, JSX } from "solid-js";

type SectionProps = {
  title: string;
  icon: JSX.Element;
  children: JSX.Element;
};

export const Section: Component<SectionProps> = (props) => (
  <div class="space-y-3">
    <h3 class="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider opacity-50">
      {props.icon}
      {props.title}
    </h3>
    <div class="space-y-3">{props.children}</div>
  </div>
);
