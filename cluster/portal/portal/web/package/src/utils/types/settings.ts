import * as UserPB from "@octelium/apis/main/userv1";

interface Settings {
  terminalWide: boolean;
  terminalFullscreen: boolean;
  terminalFontSize: number;
  itemsPerPage: number;
  navCollapsed: boolean;
  status?: UserPB.GetStatusResponse;
}

export default Settings;
