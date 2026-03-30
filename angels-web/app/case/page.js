"use client"


import TimeToggle from "@/components/TimeToggle/TimeToggle";
import { Column, Content, Grid } from "@carbon/react";

import '@/app/case/case.module.css'

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
<TimeToggle pageName="case page" keyName="23.00 - 24.00" validationName="Validation message for time1 on case" messageName="Test message for time1" />
<TimeToggle pageName="case page" keyName="time2" validationName="Validation message for time2 on case" messageName="Test message for time2" />
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
  return(<div className="backgroundContainer">
    <StoryContent/>
  </div>);
}

