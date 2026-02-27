import React, { useState, useEffect, useRef } from 'react';
import { fetchSettings, updateSettings, fetchStoreUpdate, checkMirrors, type MirrorOption, type MirrorCheckResult } from '../api/client';
import type { StoreUpdateInfo } from '../api/client';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
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

function latencyColor(result: MirrorCheckResult): string {
  if (result.status !== 'ok') return 'text-muted-foreground';
  if (result.latency_ms <= 300) return 'text-green-600';
  if (result.latency_ms <= 800) return 'text-yellow-600';
  return 'text-red-500';
}

function latencyText(result: MirrorCheckResult): string {
  if (result.status === 'timeout') return '超时';
  if (result.status === 'error') return '失败';
  return `${result.latency_ms}ms`;
}

const SettingsDialog: React.FC<SettingsDialogProps> = ({
  visible,
  onClose,
  onStoreUpdate,
}) => {
  const [interval, setInterval] = useState<number>(24);
  const [mirror, setMirror] = useState<string>('gh-proxy');
  const [mirrorOptions, setMirrorOptions] = useState<MirrorOption[]>([]);
  const [dockerMirror, setDockerMirror] = useState<string>('daocloud');
  const [dockerMirrorOptions, setDockerMirrorOptions] = useState<MirrorOption[]>([]);
  const [customGithubMirror, setCustomGithubMirror] = useState<string>('');
  const [customDockerMirror, setCustomDockerMirror] = useState<string>('');
  const [storeInfo, setStoreInfo] = useState<StoreUpdateInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

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

  useEffect(() => {
    if (visible) {
      loadData();
      setGhLatency(new Map());
      setDkLatency(new Map());
    }
  }, [visible]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [settings, store] = await Promise.all([
        fetchSettings(),
        fetchStoreUpdate()
      ]);
      setInterval(settings.check_interval_hours);
      const m = settings.mirror || 'gh-proxy';
      const dm = settings.docker_mirror || 'daocloud';
      setMirror(m);
      setMirrorOptions(settings.mirror_options || []);
      setDockerMirror(dm);
      setDockerMirrorOptions(settings.docker_mirror_options || []);
      setCustomGithubMirror(settings.custom_github_mirror || '');
      setCustomDockerMirror(settings.custom_docker_mirror || '');
      if (m !== 'direct') prevMirrorRef.current = m;
      if (dm !== 'direct') prevDockerMirrorRef.current = dm;
      setStoreInfo(store);
    } catch (error) {
      console.error('Failed to load settings:', error);
      toast.error('加载设置失败');
    } finally {
      setLoading(false);
    }
  };

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
      toast.error('GitHub 测速失败');
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
      toast.error('Docker 测速失败');
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
    setSaving(true);
    try {
      await updateSettings({
        check_interval_hours: interval,
        mirror,
        docker_mirror: dockerMirror,
        custom_github_mirror: customGithubMirror || undefined,
        custom_docker_mirror: customDockerMirror || undefined,
      });
      onClose();
    } catch (error) {
      console.error('Failed to save settings:', error);
      toast.error('保存设置失败');
    } finally {
      setSaving(false);
    }
  };

  const githubSelectOptions = mirrorOptions.filter((opt) => opt.key !== 'direct');
  const dockerSelectOptions = dockerMirrorOptions.filter((opt) => opt.key !== 'direct');

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>设置</DialogTitle>
        </DialogHeader>
        
        {loading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : (
          <div className="space-y-6 py-4">
            <div className="space-y-2">
              <label className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                自动检查更新间隔
              </label>
              <Select 
                value={interval.toString()} 
                onValueChange={(value) => setInterval(Number(value))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择间隔" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">1 小时</SelectItem>
                  <SelectItem value="6">6 小时</SelectItem>
                  <SelectItem value="12">12 小时</SelectItem>
                  <SelectItem value="24">24 小时</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <label className="text-sm font-medium leading-none">
                    GitHub 下载加速
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
                    测速
                  </Button>
                </div>
                <Switch checked={githubEnabled} onCheckedChange={handleGithubToggle} />
              </div>
              {githubEnabled && (
                <>
                  <Select
                    value={mirror}
                    onValueChange={(value) => setMirror(value)}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择镜像" />
                    </SelectTrigger>
                    <SelectContent>
                      {githubSelectOptions.map((opt) => {
                        const result = ghLatency.get(opt.key);
                        return (
                          <SelectItem key={opt.key} value={opt.key}>
                            <span className="flex items-center justify-between w-full gap-2">
                              <span>{opt.label}</span>
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
                      placeholder="https://your-proxy.example.com/"
                      value={customGithubMirror}
                      onChange={(e) => setCustomGithubMirror(e.target.value)}
                    />
                  )}
                </>
              )}
              <p className="text-xs text-muted-foreground">
                {githubEnabled ? '使用镜像加速从 GitHub 下载应用安装包' : '直接从 GitHub 下载，不使用加速'}
              </p>
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <label className="text-sm font-medium leading-none">
                    Docker 镜像加速
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
                    测速
                  </Button>
                </div>
                <Switch checked={dockerEnabled} onCheckedChange={handleDockerToggle} />
              </div>
              {dockerEnabled && (
                <>
                  <Select
                    value={dockerMirror}
                    onValueChange={(value) => setDockerMirror(value)}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择镜像" />
                    </SelectTrigger>
                    <SelectContent>
                      {dockerSelectOptions.map((opt) => {
                        const result = dkLatency.get(opt.key);
                        return (
                          <SelectItem key={opt.key} value={opt.key}>
                            <span className="flex items-center justify-between w-full gap-2">
                              <span>{opt.label}</span>
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
                      placeholder="your-mirror.example.com/"
                      value={customDockerMirror}
                      onChange={(e) => setCustomDockerMirror(e.target.value)}
                    />
                  )}
                </>
              )}
              <p className="text-xs text-muted-foreground">
                {dockerEnabled ? 'Docker 类应用拉取镜像时使用的加速源' : '直接从 Docker Hub 拉取，不使用加速'}
              </p>
            </div>

            <Separator />

            <div className="space-y-4">
              <h3 className="text-sm font-medium text-muted-foreground">商店版本</h3>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-sm">
                  <span className="text-muted-foreground">当前版本:</span>
                  <span className="font-medium">{storeInfo?.current_version || '未知'}</span>
                  {storeInfo?.has_update && (
                    <Badge variant="secondary" className="bg-primary/10 text-primary border-0 font-medium px-1.5 h-5 text-[11px] rounded-full">
                      v{storeInfo.available_version} 可用
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
                    更新商店
                  </Button>
                )}
              </div>
            </div>
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={saving}>
            取消
          </Button>
          <Button onClick={handleSave} disabled={saving || loading}>
            {saving && (
              <Loader2 className="-ml-1 mr-2 h-4 w-4 animate-spin" />
            )}
            保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default SettingsDialog;
