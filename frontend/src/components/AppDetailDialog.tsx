import React from 'react';
import type { AppInfo, AppOperation } from '../api/client';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Package,
  Globe,
  Clock,
  Tag,
  Cpu,
  Network,
  ExternalLink,
  Circle,
  Download,
  RefreshCw,
} from 'lucide-react';

interface AppDetailDialogProps {
  app: AppInfo | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onInstall: (app: AppInfo) => void;
  onUpdate: (app: AppInfo) => void;
  operation?: AppOperation;
}

const AppDetailDialog: React.FC<AppDetailDialogProps> = ({ app, open, onOpenChange, onInstall, onUpdate }) => {
  if (!app) return null;

  const isInstalled = app.installed;
  const canUpdate = isInstalled && app.has_update;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'running': return 'text-emerald-500 fill-emerald-500';
      case 'stopped': return 'text-amber-500 fill-amber-500';
      default: return 'text-muted-foreground/40';
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'running': return '运行中';
      case 'stopped': return '已停止';
      default: return status || '未安装';
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-';
    try {
      return new Date(dateStr).toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  };

  const DetailRow: React.FC<{ icon: React.ElementType; label: string; children: React.ReactNode }> = ({ icon: Icon, label, children }) => (
    <div className="flex items-start gap-3 py-2.5">
      <Icon className="h-4 w-4 mt-0.5 text-muted-foreground shrink-0" />
      <div className="flex-1 min-w-0">
        <p className="text-xs text-muted-foreground mb-0.5">{label}</p>
        <div className="text-sm text-foreground">{children}</div>
      </div>
    </div>
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <div className="flex items-center gap-3">
            {app.icon_url ? (
              <img
                src={app.icon_url}
                alt={app.display_name}
                className="w-12 h-12 rounded-xl object-cover bg-background shrink-0"
              />
            ) : (
              <div className="w-12 h-12 bg-muted/60 rounded-xl flex items-center justify-center text-muted-foreground shrink-0">
                <Package className="h-6 w-6 opacity-40" />
              </div>
            )}
            <div className="flex-1 min-w-0">
              <DialogTitle className="text-base">{app.display_name}</DialogTitle>
              <div className="flex items-center gap-2 mt-1">
                {isInstalled ? (
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <Circle className={`h-2 w-2 ${getStatusColor(app.status)}`} />
                    <span>{getStatusText(app.status)}</span>
                  </div>
                ) : (
                  <span className="text-xs text-muted-foreground/50">未安装</span>
                )}
                {canUpdate && (
                  <Badge variant="secondary" className="bg-primary/10 text-primary border-0 font-medium px-1.5 h-5 text-[11px] rounded-full">
                    有更新
                  </Badge>
                )}
              </div>
            </div>
          </div>
        </DialogHeader>

        {app.description && (
          <>
            <DialogDescription className="text-sm leading-relaxed">
              {app.description}
            </DialogDescription>
            <Separator />
          </>
        )}

        <div className="space-y-0">
          <DetailRow icon={Tag} label="版本">
            <div className="flex items-center gap-2 flex-wrap">
              <span>{isInstalled ? `v${app.installed_version}` : `v${app.latest_version}`}</span>
              {canUpdate && (
                <>
                  <span className="text-muted-foreground">→</span>
                  <span className="text-primary font-medium">v{app.available_version || app.latest_version}</span>
                </>
              )}
              {!isInstalled && (
                <span className="text-muted-foreground text-xs">(最新)</span>
              )}
            </div>
          </DetailRow>

          {app.service_port ? (
            <DetailRow icon={Network} label="服务端口">
              {app.service_port}
            </DetailRow>
          ) : null}

          <DetailRow icon={Cpu} label="支持平台">
            {app.platform || '-'}
          </DetailRow>

          <DetailRow icon={Clock} label="最近更新">
            {formatDate(app.updated_at)}
          </DetailRow>

          {app.homepage && (
            <DetailRow icon={Globe} label="官网">
              <a
                href={app.homepage}
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline inline-flex items-center gap-1 break-all"
              >
                {app.homepage.replace(/^https?:\/\//, '').replace(/\/$/, '')}
                <ExternalLink className="h-3 w-3 shrink-0" />
              </a>
            </DetailRow>
          )}

          {app.release_url && (
            <DetailRow icon={Tag} label="发布页">
              <a
                href={app.release_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary hover:underline inline-flex items-center gap-1"
              >
                GitHub Release
                <ExternalLink className="h-3 w-3 shrink-0" />
              </a>
            </DetailRow>
          )}
        </div>

        <Separator />

        <div className="flex justify-end gap-2">
          {!isInstalled ? (
            <Button
              size="sm"
              onClick={() => { onOpenChange(false); onInstall(app); }}
              className="rounded-full px-4"
            >
              <Download className="mr-1.5 h-3.5 w-3.5" />
              安装
            </Button>
          ) : canUpdate ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => { onOpenChange(false); onUpdate(app); }}
              className="rounded-full px-4 border-primary text-primary hover:bg-primary/10"
            >
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              更新
            </Button>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
};

export default AppDetailDialog;
