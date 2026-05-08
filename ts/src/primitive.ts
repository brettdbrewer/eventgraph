import { Event } from "./event.js";
import { InvalidTransitionError } from "./errors.js";
import {
  Activation, Cadence, EventId, EventType, Layer, PrimitiveId, SubscriptionPattern,
} from "./types.js";
import type { ActorId, ConversationId } from "./types.js";

// ── Lifecycle ────────────────────────────────────────────────────────────

export const Lifecycle = {
  Dormant: "dormant",
  Activating: "activating",
  Active: "active",
  Processing: "processing",
  Emitting: "emitting",
  Deactivating: "deactivating",
  Suspending: "suspending",
  Suspended: "suspended",
  Memorial: "memorial",
} as const;

const TRANSITIONS: Record<string, Set<string>> = {
  [Lifecycle.Dormant]: new Set([Lifecycle.Activating]),
  [Lifecycle.Activating]: new Set([Lifecycle.Active, Lifecycle.Dormant]),
  [Lifecycle.Active]: new Set([Lifecycle.Processing, Lifecycle.Deactivating, Lifecycle.Suspending, Lifecycle.Memorial]),
  [Lifecycle.Processing]: new Set([Lifecycle.Emitting, Lifecycle.Active]),
  [Lifecycle.Emitting]: new Set([Lifecycle.Active]),
  [Lifecycle.Deactivating]: new Set([Lifecycle.Dormant]),
  [Lifecycle.Suspending]: new Set([Lifecycle.Suspended]),
  [Lifecycle.Suspended]: new Set([Lifecycle.Activating, Lifecycle.Memorial]),
  [Lifecycle.Memorial]: new Set(),
};

export function isValidTransition(from: string, to: string): boolean {
  return TRANSITIONS[from]?.has(to) ?? false;
}

// ── Mutations ───────────────────────────────────────────────────────────

export type Mutation =
  | { kind: "addEvent"; type: EventType; source: ActorId; content: Record<string, unknown>; causes: EventId[]; conversationId: ConversationId }
  | { kind: "updateState"; primitiveId: PrimitiveId; key: string; value: unknown }
  | { kind: "updateActivation"; primitiveId: PrimitiveId; level: Activation }
  | { kind: "updateLifecycle"; primitiveId: PrimitiveId; state: string };

// ── Snapshot ─────────────────────────────────────────────────────────────

export interface PrimitiveState {
  id: PrimitiveId;
  layer: Layer;
  lifecycle: string;
  activation: Activation;
  cadence: Cadence;
  state: Record<string, unknown>;
  lastTick: number;
}

export interface Snapshot {
  tick: number;
  primitives: Map<string, PrimitiveState>;
  pendingEvents: Event[];
  recentEvents: Event[];
}

// ── Primitive interface ─────────────────────────────────────────────────

export interface Primitive {
  id(): PrimitiveId;
  layer(): Layer;
  process(tick: number, events: Event[], snapshot: Snapshot): Mutation[];
  subscriptions(): SubscriptionPattern[];
  cadence(): Cadence;
}

// ── Registry ─────────────────────────────────────────────────────────────

interface MutableState {
  activation: Activation;
  lifecycle: string;
  state: Record<string, unknown>;
  lastTick: number;
}

export class Registry {
  private readonly primitives = new Map<string, Primitive>();
  private readonly states = new Map<string, MutableState>();
  private ordered: string[] = [];

  register(p: Primitive): void {
    const key = p.id().value;
    if (this.primitives.has(key))
      throw new Error(`Primitive '${key}' already registered`);
    this.primitives.set(key, p);
    this.states.set(key, {
      activation: new Activation(0),
      lifecycle: Lifecycle.Dormant,
      state: {},
      lastTick: 0,
    });
    this.rebuildOrder();
  }

  get(id: PrimitiveId): Primitive | undefined {
    return this.primitives.get(id.value);
  }

  all(): Primitive[] {
    return this.ordered.map((k) => this.primitives.get(k)!);
  }

  get count(): number { return this.primitives.size; }

  allStates(): Map<string, PrimitiveState> {
    const result = new Map<string, PrimitiveState>();
    for (const [key, p] of this.primitives) {
      const ms = this.states.get(key)!;
      result.set(key, {
        id: p.id(), layer: p.layer(), lifecycle: ms.lifecycle,
        activation: ms.activation, cadence: p.cadence(),
        state: { ...ms.state }, lastTick: ms.lastTick,
      });
    }
    return result;
  }

  getLifecycle(id: PrimitiveId): string {
    return this.states.get(id.value)?.lifecycle ?? Lifecycle.Dormant;
  }

  setLifecycle(id: PrimitiveId, state: string): void {
    const ms = this.states.get(id.value);
    if (!ms) throw new Error(`Primitive '${id.value}' not found`);
    if (!isValidTransition(ms.lifecycle, state))
      throw new InvalidTransitionError(ms.lifecycle, state);
    ms.lifecycle = state;
  }

  activate(id: PrimitiveId): void {
    const ms = this.states.get(id.value);
    if (!ms) throw new Error(`Primitive '${id.value}' not found`);
    if (!isValidTransition(ms.lifecycle, Lifecycle.Activating))
      throw new InvalidTransitionError(ms.lifecycle, Lifecycle.Activating);
    ms.lifecycle = Lifecycle.Active;
  }

  setActivation(id: PrimitiveId, level: Activation): void {
    const ms = this.states.get(id.value);
    if (!ms) throw new Error(`Primitive '${id.value}' not found`);
    ms.activation = level;
  }

  updateState(id: PrimitiveId, key: string, value: unknown): void {
    const ms = this.states.get(id.value);
    if (!ms) throw new Error(`Primitive '${id.value}' not found`);
    ms.state[key] = structuredClone(value);
  }

  getLastTick(id: PrimitiveId): number {
    return this.states.get(id.value)?.lastTick ?? 0;
  }

  setLastTick(id: PrimitiveId, tick: number): void {
    const ms = this.states.get(id.value);
    if (ms) ms.lastTick = tick;
  }

  private rebuildOrder(): void {
    this.ordered = [...this.primitives.keys()].sort((a, b) => {
      const la = this.primitives.get(a)!.layer().value;
      const lb = this.primitives.get(b)!.layer().value;
      return la !== lb ? la - lb : a.localeCompare(b);
    });
  }
}
