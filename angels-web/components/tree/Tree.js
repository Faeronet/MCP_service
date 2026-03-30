// components/TreeViewComponent.js
import React, { useEffect, useState } from 'react';
import { TreeView, TreeNode } from 'carbon-components-react';
import { useRouter } from 'next/navigation';
import treeData from '../../public/json/tree.json'; // Adjust path if needed

const Tree = ({ expandedNodes, onToggle , onHandleSidenav}) => {
  const [treeNodes, setTreeNodes] = useState([]);
  const router = useRouter();

  useEffect(() => {
    setTreeNodes(treeData);
  }, []);

  const handleSelect = (node) => {
    
    if (node.url) {
      router.push(node.url);
      onHandleSidenav()
    }
  };
  
  const handleToggle =(node)=>
    {
      
      onToggle(node.label, !expandedNodes[node.label]);

    }

  const renderTreeNodes = (nodes) => {
    return nodes.map((node, index) => (
      <TreeNode
        label={node.label}
        key={index}
        isExpanded={expandedNodes[node.label] || false}
        onSelect={() => handleSelect(node)}
        onToggle={(event, isExpanded) => handleToggle(node)}
      >
        {node.children && renderTreeNodes(node.children)}
      </TreeNode>
    ));
  };

  return (
    <TreeView label="Tree view" hideLabel>
      {renderTreeNodes(treeNodes)}
    </TreeView>
  );
};

export default Tree;
