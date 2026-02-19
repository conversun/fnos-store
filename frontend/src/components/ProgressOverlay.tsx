import React from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Progress } from "@/components/ui/progress"
import { Button } from "@/components/ui/button"
import { formatBytes, formatSpeed } from "@/lib/utils"

interface ProgressOverlayProps {
  visible: boolean;
  message: string;
  progress: number;
  speed?: number;
  downloaded?: number;
  total?: number;
  onCancel?: () => void;
  onDismiss?: () => void;
}

const ProgressOverlay: React.FC<ProgressOverlayProps> = ({ visible, message, progress, speed, downloaded, total, onCancel, onDismiss }) => {
  return (
    <Dialog open={visible} onOpenChange={() => {}}>
      <DialogContent 
        className="sm:max-w-sm [&>button]:hidden" 
        onInteractOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>正在处理...</DialogTitle>
        </DialogHeader>
        
        <div className="flex flex-col gap-2 py-2">
          <div className="flex justify-between text-sm text-muted-foreground">
            <span>{message}</span>
            <span>{Math.round(progress)}%</span>
          </div>
          <Progress value={progress} className="w-full" />
          {speed != null && speed > 0 && (
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>{formatSpeed(speed)}</span>
              {downloaded != null && total != null && total > 0 && (
                <span>{formatBytes(downloaded)} / {formatBytes(total)}</span>
              )}
            </div>
          )}
        </div>

        {(onCancel || onDismiss) && (
          <div className="flex justify-end pt-2">
            {onCancel ? (
              <Button variant="ghost" size="sm" onClick={onCancel}>
                取消
              </Button>
            ) : (
              <Button variant="ghost" size="sm" onClick={onDismiss}>
                关闭
              </Button>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
};

export default ProgressOverlay;
