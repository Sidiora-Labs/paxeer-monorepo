"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { perfdbQuery, perfdbRestAuth } from "@/core/perfdb/client";

import { usePerfdbSelection } from "./usePerfdbSelection";
import { requirePerfdbIdentity } from "./usePerfdbAuth";

// ── Types ────────────────────────────────────────────────────────────
// Schema: GqlCompetition = { id, name, description, startAt, endAt, metric, prizePool, active, rankings }
// Schema: GqlCompetitionEntry = { rank, address, score }
// Spec called for: { startDate, endDate, status, participantCount }
//                  { value, prize }
// We expose BOTH — server-sourced fields + derived spec-compat fields.

export type CompetitionMetric = "pnl" | "volume" | "trades" | "roi" | string;
export type CompetitionStatus = "upcoming" | "active" | "ended";

export interface Competition {
  id: string;
  name: string;
  description: string | null;
  metric: CompetitionMetric;
  prizePool: string | null;
  // Spec-compat aliases
  startDate: string;
  endDate: string;
  status: CompetitionStatus;
  participantCount: number;
  // Schema-native
  startAt: string;
  endAt: string;
  active: boolean;
}

export interface CompetitionRanking {
  rank: number;
  address: string;
  value: string; // schema `score`
  prize: string | null; // not in schema; null
}

export interface CompetitionDetail extends Competition {
  rankings: CompetitionRanking[];
}

interface RawRanking {
  rank: number;
  address: string;
  score: string;
}

interface RawCompetition {
  id: string;
  name: string;
  description: string | null;
  startAt: string;
  endAt: string;
  metric: string;
  prizePool: string | null;
  active: boolean;
  rankings: RawRanking[];
}

// ── Queries ──────────────────────────────────────────────────────────
const COMPETITIONS_LIST_QUERY = /* GraphQL */ `
  query Competitions {
    competitions {
      id
      name
      description
      startAt
      endAt
      metric
      prizePool
      active
      rankings {
        rank
        address
        score
      }
    }
  }
`;

const COMPETITION_DETAIL_QUERY = /* GraphQL */ `
  query Competition($id: String!) {
    competition(id: $id) {
      id
      name
      description
      startAt
      endAt
      metric
      prizePool
      active
      rankings {
        rank
        address
        score
      }
    }
  }
`;

// ── Helpers ──────────────────────────────────────────────────────────
function deriveStatus(raw: RawCompetition): CompetitionStatus {
  if (raw.active) return "active";
  const now = Date.now();
  const start = Date.parse(raw.startAt);
  const end = Date.parse(raw.endAt);
  if (!isNaN(end) && now > end) return "ended";
  if (!isNaN(start) && now < start) return "upcoming";
  return "ended";
}

function normalize(raw: RawCompetition): Competition {
  const participantCount = raw.rankings.length;
  return {
    id: raw.id,
    name: raw.name,
    description: raw.description,
    metric: raw.metric,
    prizePool: raw.prizePool,
    startDate: raw.startAt,
    endDate: raw.endAt,
    status: deriveStatus(raw),
    participantCount,
    startAt: raw.startAt,
    endAt: raw.endAt,
    active: raw.active,
  };
}

function normalizeDetail(raw: RawCompetition): CompetitionDetail {
  const base = normalize(raw);
  const rankings: CompetitionRanking[] = (raw.rankings ?? []).map((r) => ({
    rank: r.rank,
    address: r.address,
    value: r.score,
    prize: null,
  }));
  return { ...base, participantCount: rankings.length, rankings };
}

// ── List hook ────────────────────────────────────────────────────────
interface CompetitionsListResponse {
  competitions: RawCompetition[];
}

export function usePerfdbCompetitions(): {
  competitions: Competition[];
  isLoading: boolean;
  error: Error | null;
} {
  const [competitions, setCompetitions] = useState<Competition[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);
  const mountedRef = useRef<boolean>(true);

  const fetchList = useCallback(async (): Promise<void> => {
    try {
      const res = await perfdbQuery<CompetitionsListResponse>(
        COMPETITIONS_LIST_QUERY
      );
      if (!mountedRef.current) return;
      setCompetitions((res.competitions ?? []).map(normalize));
      setError(null);
    } catch (err) {
      if (!mountedRef.current) return;
      setError(err instanceof Error ? err : new Error(String(err)));
      setCompetitions([]);
    } finally {
      if (mountedRef.current) setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    fetchList();
    return () => {
      mountedRef.current = false;
    };
  }, [fetchList]);

  return { competitions, isLoading, error };
}

// ── Detail hook ──────────────────────────────────────────────────────
interface CompetitionDetailResponse {
  competition: RawCompetition | null;
}

export function usePerfdbCompetitionDetail(id: string | null): {
  detail: CompetitionDetail | null;
  isLoading: boolean;
  error: Error | null;
  join: (address: string) => Promise<void>;
} {
  const { value: detail, isLoading, error, run, capture } = usePerfdbSelection<CompetitionDetail | null>(id, null);
  const selectedRef = useRef(id);
  selectedRef.current = id;
  const fetchDetail = useCallback((compId: string) => run(async () => {
    const res = await perfdbQuery<CompetitionDetailResponse>(COMPETITION_DETAIL_QUERY, { id: compId });
    return res.competition ? normalizeDetail(res.competition) : null;
  }, null), [run]);

  useEffect(() => {
    if (id) void fetchDetail(id);
    else void run(async () => null, null);
  }, [id, fetchDetail, run]);

  const join = useCallback(async (address: string): Promise<void> => {
    requirePerfdbIdentity(address);
    const compId = id;
    if (!compId || selectedRef.current !== compId) throw new Error("No matching competition selected.");
    const current = capture();
    await perfdbRestAuth<{ ok: boolean; }>(
      `/v2/competitions/${encodeURIComponent(compId)}/join`,
      { method: "POST", body: JSON.stringify({ address }) },
    );
    if (!current()) return;
    requirePerfdbIdentity(address);
    await fetchDetail(compId);
  }, [id, capture, fetchDetail]);

  return { detail, isLoading: Boolean(id) && isLoading, error, join };
}
