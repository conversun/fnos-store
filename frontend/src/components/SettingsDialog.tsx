import React, { useState, useEffect, useRef } from 'react';
import { fetchSettings, updateSettings, fetchStoreUpdate, checkMirrors, type MirrorOption, type MirrorCheckResult, type VolumeOption } from '../api/client';
import type { StoreUpdateInfo } from '../api/client';
import {
  Dialog,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { LocalizedDialogContent as DialogContent } from './LocalizedDialogContent';
import { LanguageSetting } from './LanguageSetting';
import { t, useLocale } from '@/lib/i18n';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { Switch } from "@/components/ui/switch"
import { Loader2, RefreshCw, Zap } from 'lucide-react'
import { toast } from 'sonner'

interface SettingsDialogProps {
  visible: boolean;
  onClose: () => void;
  onStoreUpdate?: () => void;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val >= 100 ? Math.round(val) : val.toFixed(1)} ${units[i]}`;
}

function latencyColor(result: MirrorCheckResult): string {
  if (result.status !== 'ok') return 'text-muted-foreground';
  if (result.latency_ms <= 300) return 'text-green-600';
  if (result.latency_ms <= 800) return 'text-yellow-600';
  return 'text-red-500';
}

function latencyText(result: MirrorCheckResult): string {
  if (result.status === 'timeout') return t('超时');
  if (result.status === 'error') return t('失败');
  return `${result.latency_ms}ms`;
}

const SettingsDialog: React.FC<SettingsDialogProps> = ({
  visible,
  onClose,
  onStoreUpdate,
}) => {
  useLocale();
  const [interval, setInterval] = useState<number>(24);
  const [mirror, setMirror] = useState<string>('gh-proxy');
  const [mirrorOptions, setMirrorOptions] = useState<MirrorOption[]>([]);
  const [dockerMirror, setDockerMirror] = useState<string>('daocloud');
  const [dockerMirrorOptions, setDockerMirrorOptions] = useState<MirrorOption[]>([]);
  const [customGithubMirror, setCustomGithubMirror] = useState<string>('');
  const [customDockerMirror, setCustomDockerMirror] = useState<string>('');
  const [installVolume, setInstallVolume] = useState<number>(0);
  const [volumeOptions, setVolumeOptions] = useState<VolumeOption[]>([]);
  const [storeInfo, setStoreInfo] = useState<StoreUpdateInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [accessProtection, setAccessProtection] = useState(false);
  const [accessToken, setAccessToken] = useState<string | undefined>();

  // Speed test state — independent for GitHub and Docker
  const [ghChecking, setGhChecking] = useState(false);
  const [dkChecking, setDkChecking] = useState(false);
  const [ghLatency, setGhLatency] = useState<Map<string, MirrorCheckResult>>(new Map());
  const [dkLatency, setDkLatency] = useState<Map<string, MirrorCheckResult>>(new Map());

  // Remember the last non-direct selection so toggling back ON restores it
  const prevMirrorRef = useRef<string>('gh-proxy');
  const prevDockerMirrorRef = useRef<string>('daocloud');

  const githubEnabled = mirror !== 'direct';
  const dockerEnabled = dockerMirror !== 'direct';


  // Reset speed-test results whenever the dialog opens (adjust state during render)
  const [prevVisible, setPrevVisible] = useState(visible);
  if (visible !== prevVisible) {
    setPrevVisible(visible);
    if (visible) {
      setLoading(true);
      setGhLatency(new Map());
      setDkLatency(new Map());
    }
  }

  useEffect(() => {
    if (!visible) return;
    let cancelled = false;
    const loadData = async () => {
      try {
        const [settings, store] = await Promise.all([
          fetchSettings(),
          fetchStoreUpdate()
        ]);
        if (cancelled) return;
        setInterval(settings.check_interval_hours);
        setAccessProtection(settings.access_protection ?? false);
        setAccessToken(undefined);
        const m = settings.mirror || 'gh-proxy';
        const dm = settings.docker_mirror || 'daocloud';
        setMirror(m);
        setMirrorOptions(settings.mirror_options || []);
        setDockerMirror(dm);
        setDockerMirrorOptions(settings.docker_mirror_options || []);
        setCustomGithubMirror(settings.custom_github_mirror || '');
        setCustomDockerMirror(settings.custom_docker_mirror || '');
        setInstallVolume(settings.install_volume || 0);
        setVolumeOptions(settings.volume_options || []);
        if (m !== 'direct') prevMirrorRef.current = m;
        if (dm !== 'direct') prevDockerMirrorRef.current = dm;
        setStoreInfo(store);
      } catch (error) {
        if (cancelled) return;
        console.error('Failed to load settings:', error);
        toast.error(t('加载设置失败'));
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    loadData();
    return () => { cancelled = true; };
  }, [visible]);

  const handleGhSpeedTest = async () => {
    setGhChecking(true);
    setGhLatency(new Map());
    try {
      const result = await checkMirrors('github');
      const gh = new Map<string, MirrorCheckResult>();
      for (const r of result.github_mirrors) gh.set(r.key, r);
      setGhLatency(gh);
    } catch (error) {
      console.error('GitHub speed test failed:', error);
      toast.error(t('GitHub 测速失败'));
    } finally {
      setGhChecking(false);
    }
  };

  const handleDkSpeedTest = async () => {
    setDkChecking(true);
    setDkLatency(new Map());
    try {
      const result = await checkMirrors('docker');
      const dk = new Map<string, MirrorCheckResult>();
      for (const r of result.docker_mirrors) dk.set(r.key, r);
      setDkLatency(dk);
    } catch (error) {
      console.error('Docker speed test failed:', error);
      toast.error(t('Docker 测速失败'));
    } finally {
      setDkChecking(false);
    }
  };

  const handleGithubToggle = (checked: boolean) => {
    if (checked) {
      setMirror(prevMirrorRef.current);
    } else {
      prevMirrorRef.current = mirror;
      setMirror('direct');
    }
  };

  const handleDockerToggle = (checked: boolean) => {
    if (checked) {
      setDockerMirror(prevDockerMirrorRef.current);
    } else {
      prevDockerMirrorRef.current = dockerMirror;
      setDockerMirror('direct');
    }
  };

  const handleSave = async () => {
    if (accessToken && (new TextEncoder().encode(accessToken).length < 16 || new TextEncoder().encode(accessToken).length > 256)) {
      toast.error(t('访问令牌长度须为 16–256 字节'));
      return;
    }
    setSaving(true);
    try {
      await updateSettings({
        access_token: accessToken,
        check_interval_hours: interval,
        mirror,
        docker_mirror: dockerMirror,
        custom_github_mirror: customGithubMirror || undefined,
        custom_docker_mirror: customDockerMirror || undefined,
        install_volume: installVolume,
      });
      onClose();
      if (accessToken !== undefined) window.location.reload();
    } catch (error) {
      console.error('Failed to save settings:', error);
      toast.error(t('保存设置失败'));
    } finally {
      setSaving(false);
    }
  };

  const githubSelectOptions = mirrorOptions.filter((opt) => opt.key !== 'direct');
  const dockerSelectOptions = dockerMirrorOptions.filter((opt) => opt.key !== 'direct');

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md max-h-[calc(100dvh-2rem)] flex flex-col">
        <DialogHeader className="shrink-0">
          <DialogTitle>{t('设置')}</DialogTitle>
        </DialogHeader>
        <LanguageSetting />
        
        {loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-6 py-4 overflow-y-auto flex-1 min-h-0">
            <fieldset className="flex flex-col gap-2">
              <legend className="text-sm font-medium">{t('访问保护')}</legend>
              <p className="text-xs text-muted-foreground">
                {accessProtection ? t('已启用。留空保留当前令牌；填写新令牌可重置。') : t('尚未启用。设置令牌后，所有访问均需登录。')}
                {' '}{t('用户名为 fnos，密码为访问令牌。远程访问请使用 HTTPS。')}
              </p>
              <label htmlFor="access-token" className="text-sm">{t('新访问令牌')}</label>
              <Input id="access-token" type="password" autoComplete="new-password"
                minLength={16} maxLength={256} value={accessToken ?? ''}
                placeholder={t('至少 16 个字符')}
                onChange={(event) => setAccessToken(event.target.value || undefined)} />
              {accessProtection && (
                <Button variant="outline" type="button" className="h-auto min-h-9 whitespace-normal" onClick={() => {
                  setAccessToken('');
                  setAccessProtection(false);
                }}>{t('关闭访问保护（保存后生效）')}</Button>
              )}
            </fieldset>

            <div className="space-y-2">
              <label htmlFor="check-interval" className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                {t('自动检查更新间隔')}
              </label>
              <Select 
                value={interval.toString()} 
                onValueChange={(value) => setInterval(Number(value))}
              >
                <SelectTrigger id="check-interval">
                  <SelectValue placeholder={t('选择间隔')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">{t('1 小时')}</SelectItem>
                  <SelectItem value="6">{t('6 小时')}</SelectItem>
                  <SelectItem value="12">{t('12 小时')}</SelectItem>
                  <SelectItem value="24">{t('24 小时')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {volumeOptions.length > 0 && (
              <div className="space-y-2">
                <label htmlFor="install-volume" className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                  {t('应用安装位置')}
                </label>
                <Select
                  value={installVolume.toString()}
                  onValueChange={(value) => setInstallVolume(Number(value))}
                >
                  <SelectTrigger id="install-volume">
                    <SelectValue placeholder={t('选择存储空间')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">{t('系统默认')}</SelectItem>
                    {volumeOptions.map((vol) => (
                      <SelectItem key={vol.index} value={vol.index.toString()}>
                        <span className="flex items-center gap-2">
                          <span>{t('存储空间 {index}', { index: vol.index })}</span>
                          {vol.total_bytes > 0 && (
                            <span className="text-xs text-muted-foreground">
                              {t('{free} 可用 / {total}', { free: formatBytes(vol.free_bytes), total: formatBytes(vol.total_bytes) })}
                            </span>
                          )}
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  {t('选择应用安装到哪个存储空间，默认使用系统指定的存储空间')}
                </p>
              </div>
            )}

            <div className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <label htmlFor="github-enabled" className="text-sm font-medium leading-none">
                    {t('GitHub 下载加速')}
                  </label>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2 text-xs text-muted-foreground hover:text-foreground"
                    onClick={handleGhSpeedTest}
                    disabled={ghChecking}
                  >
                    {ghChecking ? (
                      <Loader2 className="h-3 w-3 animate-spin mr-1" />
                    ) : (
                      <Zap className="h-3 w-3 mr-1" />
                    )}
                    {t('测速')}
                  </Button>
                </div>
                <Switch id="github-enabled" checked={githubEnabled} onCheckedChange={handleGithubToggle} />
              </div>
              {githubEnabled && (
                <>
                  <Select
                    value={mirror}
                    onValueChange={(value) => setMirror(value)}
                  >
                    <SelectTrigger aria-label={t('GitHub 下载加速')}>
                      <SelectValue placeholder={t('选择镜像')} />
                    </SelectTrigger>
                    <SelectContent>
                      {githubSelectOptions.map((opt) => {
                        const result = ghLatency.get(opt.key);
                        return (
                          <SelectItem key={opt.key} value={opt.key}>
                            <span className="flex items-center justify-between w-full gap-2">
                              <span>{opt.key === 'custom' ? t('自定义') : opt.label}</span>
                              {result && (
                                <span className={`text-[11px] tabular-nums ${latencyColor(result)}`}>
                                  {latencyText(result)}
                                </span>
                              )}
                            </span>
                          </SelectItem>
                        );
                      })}
                    </SelectContent>
                  </Select>
                  {mirror === 'custom' && (
                    <Input
                      aria-label={t('自定义 GitHub 镜像地址')}
                      placeholder="https://your-proxy.example.com/"
                      value={customGithubMirror}
                      onChange={(e) => setCustomGithubMirror(e.target.value)}
                    />
                  )}
                </>
              )}
              <p className="text-xs text-muted-foreground">
                {githubEnabled ? t('使用镜像加速从 GitHub 下载应用安装包') : t('直接从 GitHub 下载，不使用加速')}
              </p>
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <div className="flex min-w-0 flex-wrap items-center gap-2">
                  <label htmlFor="docker-enabled" className="text-sm font-medium leading-none">
                    {t('Docker 镜像加速')}
                  </label>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2 text-xs text-muted-foreground hover:text-foreground"
                    onClick={handleDkSpeedTest}
                    disabled={dkChecking}
                  >
                    {dkChecking ? (
                      <Loader2 className="h-3 w-3 animate-spin mr-1" />
                    ) : (
                      <Zap className="h-3 w-3 mr-1" />
                    )}
                    {t('测速')}
                  </Button>
                </div>
                <Switch id="docker-enabled" checked={dockerEnabled} onCheckedChange={handleDockerToggle} />
              </div>
              {dockerEnabled && (
                <>
                  <Select
                    value={dockerMirror}
                    onValueChange={(value) => setDockerMirror(value)}
                  >
                    <SelectTrigger aria-label={t('Docker 镜像加速')}>
                      <SelectValue placeholder={t('选择镜像')} />
                    </SelectTrigger>
                    <SelectContent>
                      {dockerSelectOptions.map((opt) => {
                        const result = dkLatency.get(opt.key);
                        return (
                          <SelectItem key={opt.key} value={opt.key}>
                            <span className="flex items-center justify-between w-full gap-2">
                              <span>{opt.key === 'custom' ? t('自定义') : opt.label}</span>
                              {result && (
                                <span className={`text-[11px] tabular-nums ${latencyColor(result)}`}>
                                  {latencyText(result)}
                                </span>
                              )}
                            </span>
                          </SelectItem>
                        );
                      })}
                    </SelectContent>
                  </Select>
                  {dockerMirror === 'custom' && (
                    <Input
                      aria-label={t('自定义 Docker 镜像地址')}
                      placeholder="your-mirror.example.com/"
                      value={customDockerMirror}
                      onChange={(e) => setCustomDockerMirror(e.target.value)}
                    />
                  )}
                </>
              )}
              <p className="text-xs text-muted-foreground">
                {dockerEnabled ? t('Docker 类应用拉取镜像时使用的加速源') : t('直接从 Docker Hub 拉取，不使用加速')}
              </p>
            </div>

            <Separator />

            <div className="space-y-4">
              <h3 className="text-sm font-medium text-muted-foreground">{t('商店版本')}</h3>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex flex-wrap items-center gap-2 text-sm">
                  <span className="text-muted-foreground">{t('当前版本:')}</span>
                  <span className="font-medium">{storeInfo?.current_version || t('未知')}</span>
                  {storeInfo?.has_update && (
                    <Badge variant="secondary" className="bg-primary/10 text-primary border-0 font-medium px-1.5 text-xs rounded-full">
                      {t('v{version} 可用', { version: storeInfo.available_version ?? '' })}
                    </Badge>
                  )}
                </div>
                {storeInfo?.has_update && onStoreUpdate && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => { onClose(); onStoreUpdate(); }}
                    className="rounded-full px-3.5 h-7 text-xs font-medium border-primary text-primary hover:bg-primary/10"
                  >
                    <RefreshCw className="mr-1 h-3.5 w-3.5" />
                    {t('更新商店')}
                  </Button>
                )}
              </div>
            </div>
          </div>
        )}

        <DialogFooter className="shrink-0">
          <Button variant="outline" onClick={onClose} disabled={saving}>
            {t('取消')}
          </Button>
          <Button onClick={handleSave} disabled={saving || loading}>
            {saving && (
              <Loader2 className="-ml-1 mr-2 h-4 w-4 animate-spin" />
            )}
            {t('保存')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default SettingsDialog;
