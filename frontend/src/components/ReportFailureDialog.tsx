import { useEffect, useState } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { toast } from 'sonner';
import { fetchDiagnostic } from '@/api/client';
import type { DiagnosticResponse } from '@/api/client';

interface ReportFailureDialogProps {
  open: boolean;
  onClose: () => void;
  app: string;
  step: string;
  errorMessage: string;
}

const stepTranslations: Record<string, string> = {
  downloading: '下载',
  pulling: '拉取镜像',
  installing: '安装',
  verifying: '验证',
  starting: '启动',
  request: '请求',
};

export function ReportFailureDialog({ open, onClose, app, step, errorMessage }: ReportFailureDialogProps) {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<DiagnosticResponse | null>(null);
  const [fetchError, setFetchError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    if (open && app) {
      // Use setTimeout to avoid synchronous setState in effect
      setTimeout(() => {
        if (!mounted) return;
        setLoading(true);
        setFetchError(null);
        setData(null);
      }, 0);
      fetchDiagnostic(app, step, errorMessage)
        .then((res) => {
          if (mounted) setData(res);
        })
        .catch((err) => {
          if (mounted) setFetchError(err instanceof Error ? err.message : String(err));
        })
        .finally(() => {
          if (mounted) setLoading(false);
        });
    } else {
      setTimeout(() => {
        if (!mounted) return;
        setLoading(false);
        setData(null);
        setFetchError(null);
      }, 0);
    }
    return () => {
      mounted = false;
    };
  }, [open, app, step, errorMessage]);

  const handleReport = () => {
    const url = data?.issue_url || 'https://github.com/conversun/fnos-apps/issues/new?template=bug-report.yml';
    window.open(url, '_blank');
  };

  const handleCopy = () => {
    if (data?.report) {
      navigator.clipboard.writeText(JSON.stringify(data.report, null, 2))
        .then(() => toast.success('已复制诊断信息'))
        .catch(() => toast.error('复制失败'));
    } else {
      navigator.clipboard.writeText(errorMessage)
        .then(() => toast.success('已复制错误信息'))
        .catch(() => toast.error('复制失败'));
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle>应用操作失败</DialogTitle>
          <DialogDescription>
            收集了以下诊断信息，请协助我们改进。
          </DialogDescription>
        </DialogHeader>

        <div className="py-4">
          {loading ? (
            <div className="flex justify-center items-center py-8">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          ) : fetchError ? (
            <div className="space-y-4">
              <p className="text-sm text-destructive">获取诊断信息失败: {fetchError}</p>
              <div className="bg-muted p-4 rounded-md">
                <p className="text-sm font-mono break-all">{errorMessage}</p>
              </div>
            </div>
          ) : data?.report ? (
            <div className="space-y-4 text-sm">
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <span className="text-muted-foreground">应用: </span>
                  <span className="font-medium">{data.report.display_name} {data.report.version ? `v${data.report.version}` : ''}</span>
                </div>
                <div>
                  <span className="text-muted-foreground">架构: </span>
                  <span className="font-medium">{data.report.arch}</span>
                </div>
                <div className="col-span-2">
                  <span className="text-muted-foreground">失败步骤: </span>
                  <span className="font-medium">{stepTranslations[data.report.failed_step] || data.report.failed_step}</span>
                </div>
                <div className="col-span-2">
                  <span className="text-muted-foreground">错误信息: </span>
                  <span className="font-medium text-destructive">{data.report.error_message}</span>
                </div>
              </div>
              
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">相关日志:</span>
                  {data.report.log_truncated && (
                    <span className="text-xs text-muted-foreground italic">日志已截断</span>
                  )}
                </div>
                <pre className="bg-muted p-4 rounded-md overflow-auto max-h-[200px] text-xs font-mono whitespace-pre-wrap">
                  {data.report.log_tail || '暂无相关日志'}
                </pre>
              </div>
            </div>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            关闭
          </Button>
          <Button variant="secondary" onClick={handleCopy}>
            {data?.report ? '复制诊断信息' : '复制错误信息'}
          </Button>
          <Button onClick={handleReport}>
            打开 GitHub 上报
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
