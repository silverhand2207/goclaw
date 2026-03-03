// Per-channel-type field definitions for credentials and config.
// Used by the form dialog to render proper UI fields instead of raw JSON.

export interface FieldDef {
  key: string;
  label: string;
  type: "text" | "password" | "number" | "boolean" | "select" | "tags";
  placeholder?: string;
  required?: boolean;
  defaultValue?: string | number | boolean | string[];
  options?: { value: string; label: string }[];
  help?: string;
}

// --- Shared option lists ---

const dmPolicyOptions = [
  { value: "pairing", label: "Pairing (require code)" },
  { value: "open", label: "Open (accept all)" },
  { value: "allowlist", label: "Allowlist only" },
  { value: "disabled", label: "Disabled" },
];

export const groupPolicyOptions = [
  { value: "open", label: "Open (accept all)" },
  { value: "pairing", label: "Pairing (require approval)" },
  { value: "allowlist", label: "Allowlist only" },
  { value: "disabled", label: "Disabled" },
];

// --- Credentials schemas ---

export const credentialsSchema: Record<string, FieldDef[]> = {
  telegram: [
    { key: "token", label: "Bot Token", type: "password", required: true, placeholder: "123456:ABC-DEF...", help: "From @BotFather" },
    { key: "proxy", label: "HTTP Proxy", type: "text", placeholder: "http://proxy:8080" },
  ],
  discord: [
    { key: "token", label: "Bot Token", type: "password", required: true, placeholder: "Discord bot token" },
  ],
  feishu: [
    { key: "app_id", label: "App ID", type: "text", required: true, placeholder: "cli_xxxxx" },
    { key: "app_secret", label: "App Secret", type: "password", required: true },
    { key: "encrypt_key", label: "Encrypt Key", type: "password", help: "For webhook mode" },
    { key: "verification_token", label: "Verification Token", type: "password", help: "For webhook mode" },
  ],
  zalo_oa: [
    { key: "token", label: "OA Access Token", type: "password", required: true },
    { key: "webhook_secret", label: "Webhook Secret", type: "password" },
  ],
  whatsapp: [
    { key: "bridge_url", label: "Bridge URL", type: "text", required: true, placeholder: "http://bridge:3000" },
  ],
};

// --- Config schemas ---

export const configSchema: Record<string, FieldDef[]> = {
  telegram: [
    { key: "dm_policy", label: "DM Policy", type: "select", options: dmPolicyOptions, defaultValue: "pairing" },
    { key: "group_policy", label: "Group Policy", type: "select", options: groupPolicyOptions, defaultValue: "open" },
    { key: "require_mention", label: "Require @mention in groups", type: "boolean", defaultValue: true },
    { key: "history_limit", label: "Group History Limit", type: "number", defaultValue: 50, help: "Max pending group messages for context (0 = disabled)" },
    { key: "stream_mode", label: "Stream Mode", type: "select", options: [{ value: "off", label: "Off" }, { value: "partial", label: "Partial (edit messages)" }], defaultValue: "off" },
    { key: "reaction_level", label: "Reaction Level", type: "select", options: [{ value: "off", label: "Off" }, { value: "minimal", label: "Minimal" }, { value: "full", label: "Full" }], defaultValue: "off" },
    { key: "media_max_bytes", label: "Max Media Size (bytes)", type: "number", defaultValue: 20971520, help: "Default: 20MB" },
    { key: "link_preview", label: "Link Preview", type: "boolean", defaultValue: true },
    { key: "allow_from", label: "Allowed Users", type: "tags", help: "User IDs or @usernames, one per line" },
  ],
  discord: [
    { key: "dm_policy", label: "DM Policy", type: "select", options: dmPolicyOptions, defaultValue: "open" },
    { key: "group_policy", label: "Group Policy", type: "select", options: groupPolicyOptions, defaultValue: "open" },
    { key: "require_mention", label: "Require @mention in groups", type: "boolean", defaultValue: true },
    { key: "history_limit", label: "Group History Limit", type: "number", defaultValue: 50, help: "Max pending group messages for context (0 = disabled)" },
    { key: "allow_from", label: "Allowed Users", type: "tags", help: "Discord user IDs" },
  ],
  feishu: [
    { key: "domain", label: "Domain", type: "select", options: [{ value: "lark", label: "Lark (global) — webhook only" }, { value: "feishu", label: "Feishu (China)" }], defaultValue: "lark" },
    { key: "connection_mode", label: "Connection Mode", type: "select", options: [{ value: "webhook", label: "Webhook" }, { value: "websocket", label: "WebSocket (Feishu only)" }], defaultValue: "webhook", help: "Lark Global only supports Webhook" },
    { key: "webhook_port", label: "Webhook Port", type: "number", defaultValue: 3000, help: "Webhook mode only" },
    { key: "webhook_path", label: "Webhook Path", type: "text", defaultValue: "/feishu/events", help: "Webhook mode only" },
    { key: "dm_policy", label: "DM Policy", type: "select", options: dmPolicyOptions, defaultValue: "pairing" },
    { key: "group_policy", label: "Group Policy", type: "select", options: groupPolicyOptions, defaultValue: "open" },
    { key: "require_mention", label: "Require @mention in groups", type: "boolean", defaultValue: true },
    { key: "topic_session_mode", label: "Topic Session Mode", type: "select", options: [{ value: "disabled", label: "Disabled" }, { value: "enabled", label: "Enabled" }], defaultValue: "disabled", help: "Use thread root_id for session isolation" },
    { key: "history_limit", label: "Group History Limit", type: "number", help: "Max pending group messages for context (0 = disabled)" },
    { key: "render_mode", label: "Render Mode", type: "select", options: [{ value: "auto", label: "Auto" }, { value: "raw", label: "Raw" }, { value: "card", label: "Card" }], defaultValue: "auto" },
    { key: "text_chunk_limit", label: "Text Chunk Limit", type: "number", defaultValue: 4000, help: "Max characters per message" },
    { key: "media_max_mb", label: "Max Media Size (MB)", type: "number", defaultValue: 30, help: "Max inbound media download size" },
    { key: "allow_from", label: "Allowed Users", type: "tags", help: "Lark open_ids (ou_...)" },
    { key: "group_allow_from", label: "Group Allowed Users", type: "tags", help: "Separate allowlist for group senders" },
  ],
  zalo_oa: [
    { key: "dm_policy", label: "DM Policy", type: "select", options: dmPolicyOptions, defaultValue: "pairing" },
    { key: "webhook_url", label: "Webhook URL", type: "text", placeholder: "https://..." },
    { key: "media_max_mb", label: "Max Media Size (MB)", type: "number", defaultValue: 5 },
    { key: "allow_from", label: "Allowed Users", type: "tags", help: "Zalo user IDs" },
  ],
  whatsapp: [
    { key: "dm_policy", label: "DM Policy", type: "select", options: dmPolicyOptions, defaultValue: "open" },
    { key: "group_policy", label: "Group Policy", type: "select", options: groupPolicyOptions, defaultValue: "open" },
    { key: "allow_from", label: "Allowed Users", type: "tags", help: "WhatsApp user IDs" },
  ],
};
