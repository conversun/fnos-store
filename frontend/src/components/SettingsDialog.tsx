import React, { useState, useEffect } from 'react';
import { fetchSettings, updateSettings, fetchStoreUpdate } from '../api/client';
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
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { Loader2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'

interface SettingsDialogProps {
  visible: boolean;
  onClose: () => void;
  onStoreUpdate?: () => void;
}

const mirrorLabels: Record<string, string> = {
  direct: '直连 GitHub',
  ghfast: 'GHFast 镜像',
  'gh-proxy': 'GH-Proxy 镜像',
};

const SettingsDialog: React.FC<SettingsDialogProps> = ({
  visible,
  onClose,
  onStoreUpdate,
}) => {
  const [interval, setInterval] = useState<number>(24);
  const [mirror, setMirror] = useState<string>('ghfast');
  const [mirrorOptions, setMirrorOptions] = useState<string[]>([]);
  const [storeInfo, setStoreInfo] = useState<StoreUpdateInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (visible) {
      loadData();
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
      setMirror(settings.mirror || 'ghfast');
      setMirrorOptions(settings.mirror_options || []);
      setStoreInfo(store);
    } catch (error) {
      console.error('Failed to load settings:', error);
      toast.error('加载设置失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await updateSettings({ check_interval_hours: interval, mirror });
      onClose();
    } catch (error) {
      console.error('Failed to save settings:', error);
      toast.error('保存设置失败');
    } finally {
      setSaving(false);
    }
  };

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

            <div className="space-y-2">
              <label className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                下载镜像
              </label>
              <Select
                value={mirror}
                onValueChange={(value) => setMirror(value)}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择镜像" />
                </SelectTrigger>
                <SelectContent>
                  {mirrorOptions.map((opt) => (
                    <SelectItem key={opt} value={opt}>
                      {mirrorLabels[opt] || opt}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                国内用户建议使用镜像加速下载
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
