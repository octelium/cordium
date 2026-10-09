const stopWords = new Set([
  "a",
  "an",
  "and",
  "are",
  "as",
  "at",
  "be",
  "by",
  "for",
  "from",
  "how",
  "i",
  "in",
  "is",
  "it",
  "me",
  "my",
  "of",
  "on",
  "or",
  "that",
  "the",
  "this",
  "to",
  "via",
  "was",
  "what",
  "which",
  "with",
]);

const synonyms: Record<string, string[]> = {
  svc: ["service"],
  sess: ["session"],
  ns: ["namespace"],
  cred: ["credential"],
  creds: ["credential"],
  idp: ["identity", "provider"],
  sso: ["identity", "provider"],
  ws: ["workspace"],
  sandbox: ["workspace"],
  sandboxe: ["workspace"],
  vm: ["workspace"],
  container: ["workspace"],
  environment: ["workspace", "space"],
  devbox: ["workspace"],
  snapshot: ["snapshot"],
  checkpoint: ["snapshot"],
  backup: ["snapshot"],
  disk: ["volume"],
  storage: ["volume"],
  project: ["space", "template"],
  org: ["space", "organization"],
  team: ["space", "membership"],
  blueprint: ["template"],
  prebuild: ["build", "template"],
  repo: ["git", "provider"],
  github: ["git", "provider"],
  gitlab: ["git", "provider"],
  port: ["share", "port"],
  expose: ["share", "port"],
  region: ["region"],
  env: ["user", "config"],
  dotfile: ["user", "config"],
  preference: ["user", "config"],
  remove: ["delete"],
  destroy: ["delete"],
  add: ["create"],
  new: ["create"],
  make: ["create"],
  show: ["get", "list"],
  fetch: ["get"],
  describe: ["get"],
  read: ["get"],
  all: ["list"],
  find: ["list"],
  search: ["list"],
  edit: ["update"],
  modify: ["update"],
  change: ["update"],
  set: ["update"],
  launch: ["start"],
  run: ["start"],
  resume: ["start"],
  halt: ["stop"],
  shutdown: ["stop"],
  pause: ["stop"],
  log: ["log"],
  health: ["health", "status"],
  whoami: ["status", "user"],
  token: ["token"],
  llm: ["llm"],
  ai: ["llm"],
  model: ["llm"],
  denied: ["deny"],
  blocked: ["deny"],
  user: ["user"],
  people: ["user"],
  member: ["membership"],
  config: ["config"],
  configuration: ["config"],
  setting: ["config"],
};

export const splitCamelCase = (text: string): string =>
  text
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1 $2");

export const stem = (token: string): string => {
  if (token.length > 4 && token.endsWith("ies")) {
    return `${token.slice(0, -3)}y`;
  }
  if (token.length > 4 && token.endsWith("sses")) {
    return token.slice(0, -2);
  }
  if (token.length > 3 && token.endsWith("s") && !token.endsWith("ss")) {
    return token.slice(0, -1);
  }
  return token;
};

export const tokenize = (text: string): string[] =>
  splitCamelCase(text)
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter((token) => token.length > 1 && !stopWords.has(token))
    .map(stem);

export interface SearchField {
  text: string;
  weight: number;
  primary?: boolean;
}

export interface SearchHit<T> {
  item: T;
  score: number;
}

interface Doc<T> {
  item: T;
  terms: Map<string, number>;
  primary: Set<string>;
  length: number;
  prior: number;
}

export class SearchIndex<T> {
  private docs: Doc<T>[] = [];
  private df = new Map<string, number>();
  private avgLength = 0;
  private k1 = 1.2;
  private b = 0.4;

  add(item: T, fields: SearchField[], prior = 1) {
    const terms = new Map<string, number>();
    const primary = new Set<string>();
    let length = 0;
    for (const field of fields) {
      for (const token of tokenize(field.text)) {
        terms.set(token, (terms.get(token) ?? 0) + field.weight);
        length += field.weight;
        if (field.primary) {
          primary.add(token);
        }
      }
    }

    for (const term of terms.keys()) {
      this.df.set(term, (this.df.get(term) ?? 0) + 1);
    }

    this.docs.push({ item, terms, primary, length, prior });
    this.avgLength =
      this.docs.reduce((acc, doc) => acc + doc.length, 0) / this.docs.length;
  }

  get size(): number {
    return this.docs.length;
  }

  private expandQuery(query: string): Map<string, number> {
    const ret = new Map<string, number>();
    const add = (term: string, weight: number) => {
      ret.set(term, Math.max(ret.get(term) ?? 0, weight));
    };

    for (const token of tokenize(query)) {
      add(token, 1);
      for (const syn of synonyms[token] ?? []) {
        add(stem(syn), 0.6);
      }
    }
    return ret;
  }

  private idf(term: string): number {
    const df = this.df.get(term) ?? 0;
    const n = this.docs.length;
    return Math.log(1 + (n - df + 0.5) / (df + 0.5));
  }

  search(query: string, limit = 10): SearchHit<T>[] {
    const queryTerms = this.expandQuery(query);
    if (queryTerms.size === 0) {
      return [];
    }

    const vocabulary = [...this.df.keys()];
    const prefixMatches = new Map<string, string[]>();
    for (const term of queryTerms.keys()) {
      if (term.length < 3 || this.df.has(term)) {
        continue;
      }
      prefixMatches.set(
        term,
        vocabulary.filter((v) => v.startsWith(term) && v !== term).slice(0, 8),
      );
    }

    const queryTokens = [...queryTerms.entries()]
      .filter(([, weight]) => weight === 1)
      .map(([term]) => term);

    const hits: SearchHit<T>[] = [];
    for (const doc of this.docs) {
      let score = 0;
      let matched = 0;
      let primaryMatched = 0;
      for (const [term, weight] of queryTerms) {
        const candidates = [term, ...(prefixMatches.get(term) ?? [])];
        let best = 0;
        for (const [i, candidate] of candidates.entries()) {
          const tf = doc.terms.get(candidate);
          if (!tf) {
            continue;
          }
          const norm =
            tf +
            this.k1 * (1 - this.b + (this.b * doc.length) / this.avgLength);
          const value =
            ((this.idf(candidate) * tf * (this.k1 + 1)) / norm) *
            weight *
            (i === 0 ? 1 : 0.5);
          best = Math.max(best, value);
        }
        if (best > 0) {
          matched++;
          score += best;
          if (weight === 1 && doc.primary.has(term)) {
            primaryMatched++;
          }
        }
      }
      if (score > 0) {
        const coverage =
          doc.primary.size > 0 && queryTokens.length > 0
            ? (primaryMatched / doc.primary.size) *
              (primaryMatched / queryTokens.length)
            : 0;
        hits.push({
          item: doc.item,
          score: (score * (1 + matched / 10) + 4 * coverage) * doc.prior,
        });
      }
    }

    return hits.sort((a, b) => b.score - a.score).slice(0, limit);
  }
}
