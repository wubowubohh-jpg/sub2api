import { apiClient } from './client'

export type SupplierStatus = 'pending' | 'approved' | 'rejected' | 'frozen'
export interface Supplier { id:number; user_id:number; name:string; relay_url:string; application_note:string; status:SupplierStatus; review_note:string; freeze_reason:string; pending_balance_cny:number; available_balance_cny:number; frozen_balance_cny:number; created_at:string }
export interface SupplierGroup { id:number; name:string; description?:string; platform:string; rate_multiplier:number; status:string; is_exclusive:boolean; sort_order:number }
export interface HallMetrics { request_count:number; avg_latency_ms?:number; avg_first_token_ms?:number; cache_hit_rate?:number; tps?:number; availability?:number; timeline:Array<{at:string;requests:number}> }
export interface HallGroup { id:number; name:string; description:string; platform:string; base_rate:number; admin_adjustment:number; effective_rate:number; supplier_id?:number; supplier_name?:string; status:string; is_exclusive:boolean; metrics:HallMetrics }
export interface SupplierSettings { global_rate_adjustment:number; minimum_withdrawal_usd:number }
export interface SupplierWithdrawal { id:number; supplier_id:number; request_no:string; amount_cny:number; method:string; status:'pending'|'approved'|'rejected'|'paid'; review_note:string; payment_proof_key?:string; created_at:string }

export const supplierAPI = {
  async me() { return (await apiClient.get<Supplier>('/suppliers/me')).data },
  async apply(payload:{name:string;relay_url:string;application_note:string}) { return (await apiClient.post<Supplier>('/suppliers/apply',payload)).data },
  async groups() { return (await apiClient.get<SupplierGroup[]>('/suppliers/groups')).data },
  async createGroup(payload:Partial<SupplierGroup>) { return (await apiClient.post<SupplierGroup>('/suppliers/groups',payload)).data },
  async updateGroup(id:number,payload:Partial<SupplierGroup>) { return (await apiClient.put<SupplierGroup>(`/suppliers/groups/${id}`,payload)).data },
  async accounts() { return (await apiClient.get<any[]>('/suppliers/accounts')).data },
  async hall(window='6h') { return (await apiClient.get<{groups:HallGroup[]}>('/supplier-hall',{params:{window}})).data },
  async withdraw(payload:{amount_cny:number;method:string;profile:Record<string,unknown>}) { return (await apiClient.post('/suppliers/withdrawals',payload)).data },
}
export const adminSupplierAPI = {
  async list(status='') { return (await apiClient.get<{items:Supplier[]}>('/admin/suppliers',{params:{status}})).data },
  async review(id:number,status:'approved'|'rejected',note:string) { return (await apiClient.put<Supplier>(`/admin/suppliers/${id}/review`,{status,note})).data },
  async freeze(id:number,reason:string) { return (await apiClient.post<Supplier>(`/admin/suppliers/${id}/freeze`,{reason})).data },
  async settings() { return (await apiClient.get<SupplierSettings>('/admin/suppliers/settings')).data },
  async updateSettings(payload:SupplierSettings) { return (await apiClient.put<SupplierSettings>('/admin/suppliers/settings',payload)).data },
  async withdrawals(status='') { return (await apiClient.get<{items:SupplierWithdrawal[]}>('/admin/supplier-withdrawals',{params:{status}})).data },
  async reviewWithdrawal(id:number,status:'approved'|'rejected'|'paid',note='',payment_proof_key='') { return (await apiClient.put<SupplierWithdrawal>(`/admin/supplier-withdrawals/${id}`,{status,note,payment_proof_key})).data },
}
