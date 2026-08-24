import { useEffect, useState } from "react";
import { configErrorMessage, useConfig, usePutConfig } from "../../api/config";
import { useUiStore } from "../../store/uiStore";

export function TaskConcurrencyEditor() {
  const config = useConfig();
  const putConfig = usePutConfig();
  const pushError = useUiStore((state) => state.pushError);
  const [value, setValue] = useState<string | null>(null);

  useEffect(() => {
    if (config.data && value === null) setValue(String(config.data.task_concurrency));
  }, [config.data, value]);

  const parsed = Number(value ?? "");
  const valid = value !== null && Number.isInteger(parsed) && parsed > 0;

  return (
    <div className="config-editor" data-ui="config-editor">
      <div className="config-editor-header" data-slot="header">
        <div>
          <h2>Dependent work</h2>
          <p>Limit how many agent runtimes tasks may start at once.</p>
        </div>
      </div>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (!valid || putConfig.isPending) return;
          putConfig.mutate(
            { task_concurrency: parsed },
            { onError: (error) => pushError("Saving task concurrency failed", configErrorMessage(error)) },
          );
        }}
      >
        <label className="form-field">
          Concurrent task runtimes
          <input
            aria-label="Concurrent task runtimes"
            min="1"
            onChange={(event) => setValue(event.target.value)}
            step="1"
            type="number"
            value={value ?? ""}
          />
        </label>
        {!valid && <p className="form-error">Enter a positive whole number.</p>}
        <button type="submit" disabled={!valid || putConfig.isPending || config.isLoading}>
          {putConfig.isPending ? "Saving…" : "Save"}
        </button>
      </form>
    </div>
  );
}
