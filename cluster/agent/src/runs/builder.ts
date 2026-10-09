import type {
  AgentEventPayload,
  Block,
  Message,
  ToolBlock,
} from "../protocol/index.ts";

export type BlockInit = Block extends infer B
  ? B extends Block
    ? Omit<B, "id" | "createdAt">
    : never
  : never;

export type EmitFn = (payload: AgentEventPayload) => void;

export class MessageBuilder {
  readonly message: Message;
  private emit: EmitFn;
  private blocks = new Map<string, Block>();
  private toolBlocks = new Map<string, string>();
  private counter = 0;
  private onChange?: () => void;

  constructor(message: Message, emit: EmitFn, onChange?: () => void) {
    this.message = message;
    this.emit = emit;
    this.onChange = onChange;
    for (const block of message.blocks) {
      this.blocks.set(block.id, block);
    }
  }

  addBlock<B extends Block>(init: BlockInit): B {
    const block = {
      ...init,
      id: `b${++this.counter}`,
      createdAt: new Date().toISOString(),
    } as B;
    this.message.blocks.push(block);
    this.blocks.set(block.id, block);
    if (block.type === "tool") {
      this.toolBlocks.set(block.toolCallId, block.id);
    }
    this.emit({
      type: "block.created",
      messageId: this.message.id,
      block: structuredClone(block),
    });
    this.onChange?.();
    return block;
  }

  appendText(blockId: string, delta: string) {
    const block = this.blocks.get(blockId);
    if (!block || delta === "") {
      return;
    }
    if (block.type !== "markdown" && block.type !== "thinking") {
      return;
    }
    block.text += delta;
    this.emit({
      type: "block.delta",
      messageId: this.message.id,
      blockId,
      text: delta,
    });
    this.onChange?.();
  }

  updateBlock<B extends Block>(
    blockId: string,
    patch: Partial<Omit<B, "id" | "type" | "createdAt">>,
  ): B | undefined {
    const block = this.blocks.get(blockId) as B | undefined;
    if (!block) {
      return undefined;
    }
    for (const [key, value] of Object.entries(patch)) {
      if (value === undefined) {
        delete (block as unknown as Record<string, unknown>)[key];
      } else {
        (block as unknown as Record<string, unknown>)[key] = value;
      }
    }
    this.emit({
      type: "block.updated",
      messageId: this.message.id,
      block: structuredClone(block),
    });
    this.onChange?.();
    return block;
  }

  getBlock<B extends Block = Block>(blockId: string): B | undefined {
    return this.blocks.get(blockId) as B | undefined;
  }

  getToolBlock(toolCallId: string): ToolBlock | undefined {
    const id = this.toolBlocks.get(toolCallId);
    return id ? (this.blocks.get(id) as ToolBlock) : undefined;
  }
}
