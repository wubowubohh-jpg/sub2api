import { apiClient } from './client'

export type SupplierStatus = 'pending' | 'approved' | 'rejected' | 'frozen'
export interface Supplier { id:number; user_id:number; name:string; relay_url:string; application_note:string; status:SupplierStatus; review_note:string; freeze_reason:string; pending_balance_cny:number; available_balance_cny:number; frozen_balance_cny:number; created_at:string }
export interface SupplierGroup { id:number; name:string; description?:string; platform:string; rate_multiplier:number; status:string; is_exclusive:boolean; sort_order:number }
export interface HallMetrics { request_count:number; avg_latency_ms?:number; avg_first_token_ms?:number; cache_hit_rate?:number; tps?:number; availability?:number; timeline:Array<{at:string;requests:number}> }
export interface HallGroup { id:number; name:string; description:string; platform:string; base_rate:number; admin_adjustment:number; effective_rate:number; supplier_id?:number; supplier_name?:string; status:string; is_exclusive:boolean; metrics:HallMetrics }
export interface SupplierSettings { global_rate_adjustment:number; minimum_withdrawal_usd:number }
export interface SupplierWithdrawal { id:number; supplier_id:number; request_no:string; amount_cny:number; method:string; status:'pending'|'approved'|'rejected'|'paid'; review_note:string; payment_proof_key?:string; created_at:string }
export interface SupplierResourceRequest { id:number; supplier_id:number; group_name:string; relay_name:string; relay_url:string; model:string; status:'pending'|'approved'|'rejected'; review_note:string; group_id?:number; account_id?:number; monitor_id?:number; created_at:string }
export interface SupplierBill { id:number; group_id:number; group_name:string; model:string; input_tokens:number; output_tokens:number; cache_read_tokens:number; base_rate:number; effective_rate:number; amount_cny:number; status:'pending'|'available'|'frozen'; available_at?:string; created_at:string }

export const supplierAPI = {
  async me() { return (await apiClient.get<Supplier>('/suppliers/me')).data },
  async apply(payload:{name:string;relay_url:string;application_note:string}) { return (await apiClient.post<Supplier>('/suppliers/apply',payload)).data },
  async groups() { return (await apiClient.get<SupplierGroup[]>('/suppliers/groups')).data },
  async createGroup(payload:Partial<SupplierGroup>) { return (await apiClient.post<SupplierGroup>('/suppliers/groups',payload)).data },
  async updateGroup(id:number,payload:Partial<SupplierGroup>) { return (await apiClient.put<SupplierGroup>(`/suppliers/groups/${id}`,payload)).data },
  async accounts() { return (await apiClient.get<any[]>('/suppliers/accounts')).data },
  async resourceRequests() { return (await apiClient.get<{items:SupplierResourceRequest[]}>('/suppliers/resource-requests')).data },
  async createResourceRequest(payload:{group_name:string;relay_name:string;relay_url:string;api_key:string;model:string}) { return (await apiClient.post<SupplierResourceRequest>('/suppliers/resource-requests',payload)).data },
  async bills(status='') { return (await apiClient.get<{items:SupplierBill[]}>('/suppliers/bills',{params:{status}})).data },
  async hall(window='6h') { return (await apiClient.get<{groups:HallGroup[]}>('/supplier-hall',{params:{window}})).data },
  async withdraw(payload:{amount_cny:number;method:string;profile:Record<string,unknown>}) { return (await apiClient.post('/suppliers/withdrawals',payload)).data },
}
export const adminSupplierAPI = {
  async list(status='') { return (await apiClient.get<{items:Supplier[]}>('/admin/suppliers',{params:{status}})).data },
  async review(id:number,status:'approved'|'rejected',note:string) { return (await apiClient.put<Supplier>(`/admin/suppliers/${id}/review`,{status,note})).data },
  async freeze(id:number,reason:string) { return (await apiClient.post<Supplier>(`/admin/suppliers/${id}/freeze`,{reason})).data },
  async unfreeze(id:number) { return (await apiClient.post<Supplier>(`/admin/suppliers/${id}/unfreeze`)).data },
  async settings() { return (await apiClient.get<SupplierSettings>('/admin/suppliers/settings')).data },
  async updateSettings(payload:SupplierSettings) { return (await apiClient.put<SupplierSettings>('/admin/suppliers/settings',payload)).data },
  async withdrawals(status='') { return (await apiClient.get<{items:SupplierWithdrawal[]}>('/admin/supplier-withdrawals',{params:{status}})).data },
  async reviewWithdrawal(id:number,status:'approved'|'rejected'|'paid',note='',payment_proof_key='') { return (await apiClient.put<SupplierWithdrawal>(`/admin/supplier-withdrawals/${id}`,{status,note,payment_proof_key})).data },
  async resourceRequests(status='') { return (await apiClient.get<{items:SupplierResourceRequest[]}>('/admin/suppliers/resource-requests',{params:{status}})).data },
  async reviewResourceRequest(id:number,approved:boolean,note='') { return (await apiClient.put<SupplierResourceRequest>(`/admin/suppliers/resource-requests/${id}`,{approved,note})).data },
}
