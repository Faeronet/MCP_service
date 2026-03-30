"use client"


import UploadData from "@/components/Upload/UploadData";
import { Column, Content, Grid } from "@carbon/react";
import { useState } from 'react';
const StoryContent = ({
  useResponsiveOffset = true
}) => {
 

  const content = <Grid>
     <Column sm={{
    span: 4,
    offset: 0
  }} md={{
    span: 8,
    offset: 0
  }} lg={{
    span: 12,
    offset: 4
  }} >
   <div>
<UploadData/>
   </div>
      </Column>
      </Grid>;
  const style = {
    height: '100%'
  };
  if (useResponsiveOffset) {
    style.margin = '0';
    style.width = '100%';
  }
  return <Content id="main-content" style={style}>
      {content}
    </Content>;
};

export default function RepoPage() {
  
  return(<div>
    <StoryContent/>
  </div>);
}
