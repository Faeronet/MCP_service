"use client"


import TimeToggle from "@/components/TimeToggle/TimeToggle";
import { Column, Grid } from "@carbon/react";
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

{/* <HomePage/> */}
<TimeToggle pageName="resource" keyName="time1" validationName="Validation message for time1 on resource" messageName="Test message for time1" />
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
  return <div>
      {content}
   </div>
};

export default function RepoPage() {
  return(<div>
    <StoryContent/>
  </div>);
}
