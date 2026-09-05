import api from '@/api'

export interface ABEvalSummary {
  total_comparisons: number
  from: string
  to: string
  model_wins: Array<{
    model: string
    wins: number
    win_rate: number
  }>
  avg_score_by_model: Array<{
    model: string
    avg_score: number
  }>
  avg_latency_by_model: Array<{
    model: string
    avg_latency_ms: number
  }>
  avg_cost_by_model: Array<{
    model: string
    avg_cost_usd: number
  }>
}

export interface ABEvalResult {
  id: number
  pipeline_id: string
  request_id: string
  question: string
  strategy: string
  winner_node: string
  candidate_a_node: string
  candidate_b_node: string
  model_a: string
  model_b: string
  score_a: number
  score_b: number
  latency_a_ms: number
  latency_b_ms: number
  cost_a_usd: number
  cost_b_usd: number
  created_at: string
}

interface ABListResponse {
  results: ABEvalResult[]
  from: string
  to: string
}

export function getABEvalSummary(params: { from?: string; to?: string } = {}): Promise<ABEvalSummary> {
  return api({ url: '/api/v1/admin/ab-eval/summary', method: 'get', params }) as Promise<ABEvalSummary>
}

export function listABEvalResults(params: { from?: string; to?: string } = {}): Promise<ABListResponse> {
  return api({ url: '/api/v1/admin/ab-eval/results', method: 'get', params }) as Promise<ABListResponse>
}
