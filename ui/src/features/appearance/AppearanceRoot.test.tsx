import React from "react";
import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { Config } from "../../schemas/config";
import { QUERY_KEYS } from "../../api/config";
import { AppearanceRoot } from "./AppearanceRoot";

const config = {
  appearance_skin: "sky-grove",
} as Config;

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute("data-skin");
});

function renderRoot(initial: Config) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  client.setQueryData(QUERY_KEYS.config, initial);
  render(
    <QueryClientProvider client={client}>
      <AppearanceRoot />
    </QueryClientProvider>,
  );
  return client;
}

describe("AppearanceRoot", () => {
  it("applies a stored built-in skin without remounting the application", async () => {
    renderRoot(config);

    await waitFor(() => expect(document.documentElement.dataset.skin).toBe("sky-grove"));
  });

  it("returns to Core when the config projection changes to an unknown skin", async () => {
    const client = renderRoot(config);
    await waitFor(() => expect(document.documentElement.dataset.skin).toBe("sky-grove"));

    client.setQueryData<Config>(QUERY_KEYS.config, { ...config, appearance_skin: "missing-skin" });

    await waitFor(() => expect(document.documentElement).not.toHaveAttribute("data-skin"));
  });
});
