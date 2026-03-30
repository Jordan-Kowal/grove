import { type Component, createSignal, Show } from "solid-js";

type BranchNameInputProps = {
  placeholder: string;
  forbiddenNames: string[];
  onSubmit: (name: string) => void;
  onCancel: () => void;
  allowSlash?: boolean;
};

export const BranchNameInput: Component<BranchNameInputProps> = (props) => {
  const [value, setValue] = createSignal("");

  const isNameTaken = () => {
    const name = value().trim();
    if (!name) return false;
    const lower = name.toLowerCase();
    return props.forbiddenNames.some((n) => n.toLowerCase() === lower);
  };

  const handleSubmit = () => {
    const name = value().trim();
    if (name && !isNameTaken()) {
      props.onSubmit(name);
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") handleSubmit();
    if (e.key === "Escape") props.onCancel();
  };

  return (
    <div>
      <input
        ref={(el) => setTimeout(() => el.focus(), 0)}
        type="text"
        class="input input-xs input-bordered w-full text-xs"
        classList={{ "input-error": isNameTaken() }}
        placeholder={props.placeholder}
        value={value()}
        onInput={(e) => {
          const pattern = props.allowSlash
            ? /[^a-zA-Z0-9\-_/]/g
            : /[^a-zA-Z0-9\-_]/g;
          const sanitized = e.currentTarget.value.replace(pattern, "");
          e.currentTarget.value = sanitized;
          setValue(sanitized);
        }}
        onKeyDown={handleKeyDown}
      />
      <Show when={isNameTaken()}>
        <p class="text-[9px] text-error mt-0.5">Name already taken</p>
      </Show>
    </div>
  );
};
