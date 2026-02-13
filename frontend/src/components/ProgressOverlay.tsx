import React from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Progress } from "@/components/ui/progress"
import { Button } from "@/components/ui/button"

interface ProgressOverlayProps {
  visible: boolean;
  message: string;
  progress: number;
  onCancel?: () => void;
}

const ProgressOverlay: React.FC<ProgressOverlayProps> = ({ visible, message, progress, onCancel }) => {
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
        </div>

        {onCancel && (
          <div className="flex justify-end pt-2">
            <Button variant="ghost" size="sm" onClick={onCancel}>
              取消
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
};

export default ProgressOverlay;
