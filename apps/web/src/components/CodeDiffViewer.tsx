import React, { useState, useMemo } from 'react';
import { Tree, Spin, Empty, Typography } from 'antd';
import { Diff, parseDiff, Hunk } from 'react-diff-view';
import 'react-diff-view/style/index.css';

const { Title, Text } = Typography;

export interface ChangeItem {
  filePath: string;
  changeType: 'modify' | 'create' | 'delete';
  reason: string;
  originalContent: string;
  proposedPatch: string;
  proposedDiff: string;
  contextIncluded: boolean;
}

interface CodeDiffViewerProps {
  changeSet: ChangeItem[];
  loading?: boolean;
  summary?: string;
}

const CodeDiffViewer: React.FC<CodeDiffViewerProps> = ({
  changeSet = [],
  loading = false,
  summary
}) => {
  const [selectedFile, setSelectedFile] = useState<string>(
    changeSet.length > 0 ? changeSet[0].filePath : ''
  );

  if (loading) {
    return (
      <div className="flex justify-center items-center h-[600px]">
        <Spin size="large" tip="加载代码变更中..." />
      </div>
    );
  }

  if (changeSet.length === 0) {
    return (
      <div className="flex justify-center items-center h-[600px]">
        <Empty description="暂无代码变更" />
      </div>
    );
  }

  // 构建文件树
  const fileTree = changeSet.map(item => {
    const changeTypeLabel = item.changeType === 'modify' ? '修改'
      : item.changeType === 'create' ? '新增'
      : '删除';

    return {
      title: (
        <span>
          {item.filePath}
          <Text type="secondary" className="ml-2 text-xs">
            ({changeTypeLabel})
          </Text>
        </span>
      ),
      key: item.filePath,
      isLeaf: true,
    };
  });

  const selectedChange = changeSet.find(item => item.filePath === selectedFile);

  // 手动构建diff格式
  const diffText = useMemo(() => {
    if (!selectedChange) return '';

    const { originalContent, proposedPatch, filePath } = selectedChange;
    return `diff --git a/${filePath} b/${filePath}
--- a/${filePath}
+++ b/${filePath}
${generateUnifiedDiff(originalContent || '', proposedPatch || '')}`;
  }, [selectedChange]);

  const files = useMemo(() => {
    if (!diffText) return [];
    try {
      return parseDiff(diffText);
    } catch (e) {
      console.error('解析diff失败:', e);
      return [];
    }
  }, [diffText]);

  return (
    <div className="w-full">
      {summary && (
        <div className="mb-4 p-3 bg-gray-50 rounded">
          <Title level={5} className="mb-2">变更摘要</Title>
          <Text>{summary}</Text>
        </div>
      )}

      <div className="flex h-[600px] border rounded">
        {/* 左侧文件树 */}
        <div className="w-1/4 border-r p-2 overflow-auto">
          <Title level={5} className="mb-3">变更文件 ({changeSet.length})</Title>
          <Tree
            treeData={fileTree}
            onSelect={([key]) => setSelectedFile(key as string)}
            defaultSelectedKeys={[selectedFile]}
            showLine={true}
          />
        </div>

        {/* 右侧diff对比 */}
        <div className="w-3/4 overflow-auto">
          {selectedChange && files.length > 0 && (
            <Diff
              viewType="split"
              diffType="modify"
              hunks={files[0].hunks}
              className="!max-h-[600px]"
            >
              {(hunks) => hunks.map(hunk => <Hunk key={hunk.content} hunk={hunk} />)}
            </Diff>
          )}
        </div>
      </div>
    </div>
  );
};

// 生成统一diff格式
function generateUnifiedDiff(oldStr: string, newStr: string): string {
  const oldLines = oldStr.split('\n');
  const newLines = newStr.split('\n');

  const diffLines: string[] = [];
  let i = 0, j = 0;

  while (i < oldLines.length || j < newLines.length) {
    if (i < oldLines.length && j < newLines.length && oldLines[i] === newLines[j]) {
      diffLines.push(` ${oldLines[i]}`);
      i++;
      j++;
    } else {
      // 查找下一个匹配行
      let matchFound = false;
      for (let k = i; k < oldLines.length; k++) {
        const matchIndex = newLines.indexOf(oldLines[k], j);
        if (matchIndex !== -1) {
          // 输出删除的行
          while (i < k) {
            diffLines.push(`-${oldLines[i]}`);
            i++;
          }
          // 输出新增的行
          while (j < matchIndex) {
            diffLines.push(`+${newLines[j]}`);
            j++;
          }
          matchFound = true;
          break;
        }
      }

      if (!matchFound) {
        // 没有更多匹配，输出剩余行
        while (i < oldLines.length) {
          diffLines.push(`-${oldLines[i]}`);
          i++;
        }
        while (j < newLines.length) {
          diffLines.push(`+${newLines[j]}`);
          j++;
        }
      }
    }
  }

  return `@@ -1,${oldLines.length} +1,${newLines.length} @@
${diffLines.join('\n')}`;
}

export default CodeDiffViewer;
