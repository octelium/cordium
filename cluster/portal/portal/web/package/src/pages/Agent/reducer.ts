import type { AgentEvent, Block, Message, Run } from "@/apis/agent/protocol";

export interface ChatState {
  messages: Message[];
  run?: Run;
  lastSeq: number;
}

export const emptyChatState: ChatState = {
  messages: [],
  lastSeq: 0,
};

const upsertMessage = (messages: Message[], message: Message): Message[] => {
  const idx = messages.findIndex((itm) => itm.id === message.id);
  if (idx < 0) {
    return [...messages, message];
  }
  const ret = [...messages];
  ret[idx] = message;
  return ret;
};

const updateMessage = (
  messages: Message[],
  id: string,
  fn: (message: Message) => Message,
): Message[] => messages.map((itm) => (itm.id === id ? fn(itm) : itm));

const appendText = (block: Block, text: string): Block => {
  switch (block.type) {
    case "markdown":
    case "thinking":
      return { ...block, text: block.text + text };
    default:
      return block;
  }
};

export const applyEvent = (state: ChatState, event: AgentEvent): ChatState => {
  const isNewRun = state.run?.id !== event.runId;
  if (!isNewRun && event.seq <= state.lastSeq) {
    return state;
  }

  const next: ChatState = { ...state, lastSeq: event.seq };

  switch (event.type) {
    case "run.started":
    case "run.updated":
    case "run.completed":
    case "run.failed":
    case "run.cancelled":
      return { ...next, run: event.run };
    case "message.created":
    case "message.completed":
      return { ...next, messages: upsertMessage(next.messages, event.message) };
    case "block.created":
      return {
        ...next,
        messages: updateMessage(next.messages, event.messageId, (m) => ({
          ...m,
          blocks: [
            ...m.blocks.filter((b) => b.id !== event.block.id),
            event.block,
          ],
        })),
      };
    case "block.updated":
      return {
        ...next,
        messages: updateMessage(next.messages, event.messageId, (m) => ({
          ...m,
          blocks: m.blocks.map((b) =>
            b.id === event.block.id ? event.block : b,
          ),
        })),
      };
    case "block.delta":
      return {
        ...next,
        messages: updateMessage(next.messages, event.messageId, (m) => ({
          ...m,
          blocks: m.blocks.map((b) =>
            b.id === event.blockId ? appendText(b, event.text) : b,
          ),
        })),
      };
    default:
      return next;
  }
};
