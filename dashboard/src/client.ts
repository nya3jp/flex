import {FlexClient} from 'flex-client';
import React, {useContext} from 'react';

export const flexUrl = 'http://localhost:7111';
export const FlexClientContext = React.createContext(new FlexClient(flexUrl));

export function useFlexClient(): FlexClient {
  return useContext(FlexClientContext)
}
