import { useEffect, useState, useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Rss, Play, Plus, Trash2, Power, AlertTriangle,
  ChevronDown,
} from 'lucide-react';
import { Button, Badge, Modal, Input, Textarea, EmptyState } from '@/components/ui';
import {
  listSubscriptions,
  createSubscription,
  updateSubscription,
  deleteSubscription,
  enableSubscription,
  disableSubscription,
  runSubscription,
  type Subscription,
} from '@/api/subscription';
import { listProjects, type ProjectSummary } from '@/api/projects';
import { listSessions, type Session } from '@/api/sessions';
import { formatTime, cn } from '@/lib/utils';

/* ── Interval presets ── */
interface IntervalPreset {
  label: string;
  labelZh: string;
  value: string;
}

const INTERVAL_PRESETS: IntervalPreset[] = [
  { label: 'Every 5 min',   labelZh: '每 5 分钟',  value: '5m'  },
  { label: 'Every 15 min',  labelZh: '每 15 分钟', value: '15m' },
  { label: 'Every 30 min',  labelZh: '每 30 分钟', value: '30m' },
  { label: 'Every hour',    labelZh: '每小时',     value: '1h'  },
  { label: 'Every 2 hours', labelZh: '每 2 小时',  value: '2h'  },
  { label: 'Every 6 hours', labelZh: '每 6 小时',  value: '6h'  },
  { label: 'Every 12 hours',labelZh: '每 12 小时', value: '12h' },
  { label: 'Daily',         labelZh: '每天',       value: '24h' },
];

const CUSTOM_VALUE = '__custom__';

function IntervalPicker({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const { i18n } = useTranslation();
  const isZh = i18n.language?.startsWith('zh');
  const isPreset = INTERVAL_PRESETS.some(p => p.value === value);
  const [custom, setCustom] = useState(!isPreset && !!value);

  const selectValue = custom ? CUSTOM_VALUE : value;

  const handleSelect = (v: string) => {
    if (v === CUSTOM_VALUE) {
      setCustom(true);
    } else {
      setCustom(false);
      onChange(v);
    }
  };

  return (
    <div className="space-y-2">
      <div className="relative">
        <select
          value={selectValue}
          onChange={e => handleSelect(e.target.value)}
          className={cn(
            'w-full px-3 py-2 text-sm rounded-lg transition-all duration-200 appearance-none pr-8',
            'border border-gray-300/90 dark:border-white/[0.1]',
            'bg-white/90 backdrop-blur-sm dark:bg-[rgba(0,0,0,0.45)]',
            'text-gray-900 dark:text-white',
            'focus:outline-none focus:ring-2 focus:ring-accent/45 focus:border-accent',
          )}
        >
          <option value="" disabled>{isZh ? '选择扫描间隔' : 'Select interval'}</option>
          {INTERVAL_PRESETS.map(p => (
            <option key={p.value} value={p.value}>
              {isZh ? p.labelZh : p.label} ({p.value})
            </option>
          ))}
          <option value={CUSTOM_VALUE}>{isZh ? '✏ 自定义间隔' : '✏ Custom interval'}</option>
        </select>
        <ChevronDown size={14} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
      </div>
      {custom && (
        <Input
          placeholder="5m, 1h, 2h30m..."
          value={value}
          onChange={e => onChange(e.target.value)}
          className="font-mono text-xs"
        />
      )}
    </div>
  );
}

/* ── Select dropdown ── */
function Select({ label, value, onChange, options, placeholder }: {
  label?: string;
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
  placeholder?: string;
}) {
  return (
    <div className="space-y-1.5">
      {label && <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{label}</label>}
      <div className="relative">
        <select
          value={value}
          onChange={e => onChange(e.target.value)}
          className={cn(
            'w-full px-3 py-2 text-sm rounded-lg transition-all duration-200 appearance-none pr-8',
            'border border-gray-300/90 dark:border-white/[0.1]',
            'bg-white/90 backdrop-blur-sm dark:bg-[rgba(0,0,0,0.45)]',
            'text-gray-900 dark:text-white',
            'focus:outline-none focus:ring-2 focus:ring-accent/45 focus:border-accent',
          )}
        >
          {placeholder && <option value="">{placeholder}</option>}
          {options.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
        </select>
        <ChevronDown size={14} className="absolute right-2.5 top-1/2 -translate-y-1/2 text-gray-400 pointer-events-none" />
      </div>
    </div>
  );
}

/* ── Toggle ── */
function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label?: string }) {
  return (
    <label className="inline-flex items-center gap-2 cursor-pointer">
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={cn(
          'relative w-9 h-5 rounded-full transition-colors duration-200 shrink-0',
          checked ? 'bg-accent' : 'bg-gray-300 dark:bg-gray-600',
        )}
      >
        <span className={cn(
          'block w-3.5 h-3.5 rounded-full bg-white shadow-sm transition-transform duration-200',
          checked ? 'translate-x-[18px]' : 'translate-x-[3px]',
          'mt-[3px]',
        )} />
      </button>
      {label && <span className="text-sm text-gray-700 dark:text-gray-300">{label}</span>}
    </label>
  );
}

/* ── Subscription form type ── */
interface SubForm {
  project: string;
  chat_id: string;
  filter: string;
  exclude_filter: string;
  prompt: string;
  interval: string;
  session_key: string;
  enabled: boolean;
}

const emptyForm: SubForm = {
  project: '', chat_id: '', filter: '', exclude_filter: '',
  prompt: '', interval: '', session_key: '', enabled: true,
};

/* ── Main page ── */
export default function SubscriptionList() {
  const { t } = useTranslation();
  const [subs, setSubs] = useState<Subscription[]>([]);
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [loading, setLoading] = useState(true);

  const [editSub, setEditSub] = useState<Subscription | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState<SubForm>({ ...emptyForm });
  const [saving, setSaving] = useState(false);
  const [triggeringId, setTriggeringId] = useState<string | null>(null);
  const [sessionKeys, setSessionKeys] = useState<string[]>([]);

  const isEdit = !!editSub;

  const projectOptions = useMemo(
    () => projects.map(p => ({ value: p.name, label: p.name })),
    [projects],
  );

  useEffect(() => {
    if (!form.project) { setSessionKeys([]); return; }
    let cancelled = false;
    listSessions(form.project).then(data => {
      if (cancelled) return;
      const keys = new Set<string>();
      for (const s of data.sessions || []) {
        if (s.session_key) keys.add(s.session_key);
      }
      setSessionKeys([...keys]);
    }).catch(() => { if (!cancelled) setSessionKeys([]); });
    return () => { cancelled = true; };
  }, [form.project]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [subData, projData] = await Promise.all([listSubscriptions(), listProjects()]);
      setSubs(subData.subscriptions || []);
      setProjects(projData.projects || []);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    const handler = () => fetchData();
    window.addEventListener('cc:refresh', handler);
    return () => window.removeEventListener('cc:refresh', handler);
  }, [fetchData]);

  const openAdd = () => {
    setEditSub(null);
    setForm({ ...emptyForm });
    setShowForm(true);
  };

  const openEdit = (sub: Subscription) => {
    setEditSub(sub);
    setForm({
      project: sub.project,
      chat_id: sub.chat_id,
      filter: sub.filter || '',
      exclude_filter: sub.exclude_filter || '',
      prompt: sub.prompt,
      interval: sub.interval,
      session_key: sub.session_key,
      enabled: sub.enabled,
    });
    setShowForm(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      if (isEdit && editSub) {
        const updates: Record<string, any> = {};
        if (form.chat_id !== editSub.chat_id) updates.chat_id = form.chat_id;
        if (form.filter !== (editSub.filter || '')) updates.filter = form.filter;
        if (form.exclude_filter !== (editSub.exclude_filter || '')) updates.exclude_filter = form.exclude_filter;
        if (form.prompt !== editSub.prompt) updates.prompt = form.prompt;
        if (form.interval !== editSub.interval) updates.interval = form.interval;
        if (form.project !== editSub.project) updates.project = form.project;
        if (form.session_key !== editSub.session_key) updates.session_key = form.session_key;
        if (form.enabled !== editSub.enabled) updates.enabled = form.enabled;
        if (Object.keys(updates).length > 0) {
          await updateSubscription(editSub.id, updates);
        }
      } else {
        const body: any = { ...form };
        if (!body.filter) delete body.filter;
        if (!body.exclude_filter) delete body.exclude_filter;
        if (!body.session_key) delete body.session_key;
        await createSubscription(body);
      }
      setShowForm(false);
      fetchData();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t('common.confirmDelete'))) return;
    await deleteSubscription(id);
    fetchData();
  };

  const handleToggleEnabled = async (sub: Subscription) => {
    try {
      if (sub.enabled) {
        await disableSubscription(sub.id);
      } else {
        await enableSubscription(sub.id);
      }
      fetchData();
    } catch (e: any) {
      alert(e.message);
    }
  };

  const handleRunNow = async (sub: Subscription) => {
    setTriggeringId(sub.id);
    try {
      await runSubscription(sub.id);
      alert(t('subscription.triggerPending'));
      fetchData();
    } catch (e: any) {
      alert(e.message);
    } finally {
      setTriggeringId(null);
    }
  };

  if (loading && subs.length === 0) {
    return <div className="flex items-center justify-center h-64 text-gray-400 animate-pulse">Loading...</div>;
  }

  return (
    <div className="space-y-4 animate-fade-in">
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{t('subscription.title')}</h2>
        <Button onClick={openAdd}><Plus size={16} /> {t('subscription.add')}</Button>
      </div>

      {subs.length === 0 ? (
        <EmptyState message={t('subscription.noSubs')} icon={Rss} />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {subs.map(sub => (
            <div
              key={sub.id}
              onClick={() => openEdit(sub)}
              className={cn(
                'relative p-4 rounded-xl border transition-all cursor-pointer group',
                'bg-white dark:bg-white/[0.02]',
                sub.enabled
                  ? 'border-gray-200/80 dark:border-white/[0.06] hover:border-accent/40 hover:shadow-md hover:shadow-accent/5'
                  : 'border-dashed border-gray-300/60 dark:border-white/[0.04] opacity-60 hover:opacity-80',
              )}
            >
              {/* Header */}
              <div className="flex items-center gap-2 mb-2">
                <Rss size={14} className="text-accent shrink-0" />
                <span className="font-medium text-sm text-gray-900 dark:text-white truncate">
                  {sub.chat_name || sub.chat_id}
                </span>
              </div>

              {/* Interval badge + status */}
              <div className="flex items-center gap-2 mb-3">
                <span className="inline-flex items-center gap-1 text-[11px] font-mono bg-accent/10 text-accent px-2 py-0.5 rounded-md">
                  {sub.interval}
                </span>
                <Badge variant={sub.enabled ? 'success' : 'default'} className="text-[10px] px-1.5 py-0">
                  {sub.enabled ? t('subscription.enabled') : t('subscription.disabled')}
                </Badge>
                {(sub.consecutive_errors || 0) > 0 && (
                  <Badge variant="danger" className="text-[10px] px-1.5 py-0">
                    <AlertTriangle size={9} className="mr-0.5" />
                    {sub.consecutive_errors}
                  </Badge>
                )}
              </div>

              {/* Info */}
              <div className="space-y-1 text-xs text-gray-500 dark:text-gray-400">
                <div className="flex items-center gap-1.5">
                  <span className="font-medium w-12 shrink-0 text-gray-400">{t('subscription.project')}</span>
                  <span className="truncate">{sub.project}</span>
                </div>
                {sub.filter && (
                  <div className="flex items-start gap-1.5">
                    <span className="font-medium w-12 shrink-0 text-gray-400">{t('subscription.filter')}</span>
                    <span className="line-clamp-2">{sub.filter}</span>
                  </div>
                )}
                <div className="flex items-start gap-1.5">
                  <span className="font-medium w-12 shrink-0 text-gray-400">{t('subscription.prompt')}</span>
                  <span className="line-clamp-2">{sub.prompt}</span>
                </div>
                {sub.last_run && (
                  <div className="flex items-center gap-1.5 pt-1 border-t border-gray-100 dark:border-white/[0.04] mt-1">
                    <span className="font-medium w-12 shrink-0 text-gray-400">{t('subscription.lastRun')}</span>
                    <span>{formatTime(sub.last_run!)}</span>
                  </div>
                )}
              </div>

              {sub.last_error && (
                <p className="text-[11px] text-red-500 mt-2 line-clamp-1">{sub.last_error}</p>
              )}

              {/* Action buttons (top right) */}
              <div
                className="absolute top-3 right-3 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
                onClick={e => e.stopPropagation()}
              >
                <button
                  onClick={() => handleRunNow(sub)}
                  disabled={triggeringId === sub.id}
                  className={cn(
                    'p-1.5 rounded-lg transition-colors',
                    'text-sky-500 hover:bg-sky-50 dark:hover:bg-sky-900/20 disabled:opacity-50 disabled:cursor-wait',
                  )}
                  title={t('subscription.trigger')}
                >
                  <Play size={14} />
                </button>
                <button
                  onClick={() => handleToggleEnabled(sub)}
                  className={cn(
                    'p-1.5 rounded-lg transition-colors',
                    sub.enabled
                      ? 'text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/20'
                      : 'text-gray-400 hover:bg-gray-100 dark:hover:bg-white/[0.06]',
                  )}
                  title={sub.enabled ? t('subscription.disable') : t('subscription.enable')}
                >
                  <Power size={14} />
                </button>
                <button
                  onClick={() => handleDelete(sub.id)}
                  className="p-1.5 rounded-lg text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                  title={t('subscription.delete')}
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add / Edit modal */}
      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title={isEdit ? t('subscription.editSub') : t('subscription.add')}
        className="max-w-xl"
      >
        <div className="space-y-4">
          <Select
            label={t('subscription.project')}
            value={form.project}
            onChange={v => setForm({ ...form, project: v })}
            options={projectOptions}
            placeholder={t('subscription.selectProject')}
          />

          <Input
            label={t('subscription.chatId')}
            value={form.chat_id}
            onChange={e => setForm({ ...form, chat_id: e.target.value })}
            placeholder={t('subscription.chatIdPlaceholder')}
          />

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1.5">
              {t('subscription.interval')}
            </label>
            <IntervalPicker value={form.interval} onChange={v => setForm({ ...form, interval: v })} />
          </div>

          <Input
            label={`${t('subscription.filter')} (${t('common.optional')})`}
            value={form.filter}
            onChange={e => setForm({ ...form, filter: e.target.value })}
            placeholder={t('subscription.filterPlaceholder')}
          />

          <Input
            label={`${t('subscription.excludeFilter')} (${t('common.optional')})`}
            value={form.exclude_filter}
            onChange={e => setForm({ ...form, exclude_filter: e.target.value })}
            placeholder={t('subscription.excludeFilterPlaceholder')}
          />

          <Textarea
            label={t('subscription.prompt')}
            value={form.prompt}
            onChange={e => setForm({ ...form, prompt: e.target.value })}
            rows={3}
            placeholder={t('subscription.promptPlaceholder')}
          />

          <Select
            label={`${t('subscription.sessionKey')} (${t('common.optional')})`}
            value={form.session_key}
            onChange={v => setForm({ ...form, session_key: v })}
            options={sessionKeys.map(k => ({ value: k, label: k }))}
            placeholder={t('subscription.selectSessionKey')}
          />

          <div className="flex items-center gap-6 pt-1">
            <Toggle checked={form.enabled} onChange={v => setForm({ ...form, enabled: v })} label={t('subscription.enabled')} />
          </div>

          <div className="flex justify-end gap-2 pt-3 border-t border-gray-100 dark:border-white/[0.06]">
            <Button variant="secondary" onClick={() => setShowForm(false)}>{t('common.cancel')}</Button>
            <Button onClick={handleSave} loading={saving}>
              {isEdit ? t('common.save') : t('subscription.add')}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
