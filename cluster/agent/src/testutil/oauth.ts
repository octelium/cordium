import {
  fauxProvider,
  type OAuthAuth,
  type Provider,
} from "@earendil-works/pi-ai";

export const fakeOAuth: OAuthAuth = {
  name: "Fake (subscription)",
  isSubscription: true,
  loginLabel: "Sign in with Fake",
  async login(interaction, options) {
    interaction.notify({
      type: "auth_url",
      url: "https://auth.example.com/authorize",
      instructions: "Open the URL",
    });
    const method = await interaction.prompt({
      type: "select",
      message: "Select the login method",
      options: [
        { id: "browser", label: "Browser" },
        { id: "code", label: "Copy code" },
      ],
    });
    const code = await interaction.prompt({
      type: "manual_code",
      message: "Paste the code",
      placeholder: "code#state",
      signal: interaction.signal,
    });
    if (code !== "good") {
      throw new Error("Invalid authorization code");
    }
    interaction.notify({ type: "progress", message: "Exchanging the code" });
    return {
      type: "oauth",
      access: `token-${method}-${options?.getDeviceId?.()}`,
      refresh: "refresh",
      expires: Date.now() + 3600_000,
    };
  },
  async refresh(credential) {
    return credential;
  },
  async toAuth(credential) {
    return { apiKey: credential.access };
  },
};

export const createFakeOAuthProvider = (id = "fake-oauth"): Provider => {
  const faux = fauxProvider({
    provider: id,
    models: [{ id: `${id}-small` }, { id: `${id}-large` }],
  });
  return {
    ...faux.provider,
    id,
    name: "Fake",
    auth: { oauth: fakeOAuth },
  };
};
