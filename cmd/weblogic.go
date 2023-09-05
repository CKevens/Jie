package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yhy0/Jie/conf"
	"github.com/yhy0/Jie/pkg/protocols/httpx"
	"github.com/yhy0/Jie/scan/java/weblogic"
	"github.com/yhy0/logging"
)

/**
   @author yhy
   @since 2023/8/20
   @desc //TODO
**/

var (
	name int
	mode string
	cmd  string
)

var webLogicCmd = &cobra.Command{
	Use:   "weblogic",
	Short: "WebLogic scan && exp",
	Run: func(cmd *cobra.Command, args []string) {
		logging.New(conf.GlobalConfig.Options.Debug, "", "Jie", false)
		// 初始化 session ,todo 后续优化一下，不同网站共用一个不知道会不会出问题，应该不会
		httpx.NewSession()
		if mode == "scan" {
			switch name {
			case 1:
				weblogic.CVE_2014_4210(conf.GlobalConfig.Options.Target)
			case 2:
				weblogic.CVE_2017_3506(conf.GlobalConfig.Options.Target)
			case 3:
				weblogic.CVE_2017_10271(conf.GlobalConfig.Options.Target)
			case 4:
				weblogic.CVE_2018_2894(conf.GlobalConfig.Options.Target)
			case 5:
				weblogic.CVE_2019_2725(conf.GlobalConfig.Options.Target)
			case 6:
				weblogic.CVE_2019_2729(conf.GlobalConfig.Options.Target)
			case 7:
				weblogic.CVE_2020_2883(conf.GlobalConfig.Options.Target)
			case 8:
				weblogic.CVE_2020_14882(conf.GlobalConfig.Options.Target)
			case 9:
				weblogic.CVE_2020_14883(conf.GlobalConfig.Options.Target)
			case 10:
				weblogic.CVE_2021_2109(conf.GlobalConfig.Options.Target)
			case 0:
				weblogic.CVE_2014_4210(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2017_3506(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2017_10271(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2018_2894(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2019_2725(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2019_2729(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2020_2883(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2020_14882(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2020_14883(conf.GlobalConfig.Options.Target)
				weblogic.CVE_2021_2109(conf.GlobalConfig.Options.Target)
			}
		}
	},
}

func webLogicCmdInit() {
	rootCmd.AddCommand(webLogicCmd)
	webLogicCmd.Flags().IntVarP(&name, "name", "n", 0, "vul name: 1:CVE_2014_4210, 2:CVE_2017_3506, 3:CVE_2017_10271, 4:CVE_2018_2894, 5:CVE_2019_2725, 6:CVE_2019_2729, 7:CVE_2020_2883, 8:CVE_2020_14882, 9:CVE_2020_14883, 10:CVE_2021_2109, 0:all")
	webLogicCmd.Flags().StringVarP(&mode, "mode", "m", "scan", "Specify work mode: scan exp")
	webLogicCmd.Flags().StringVarP(&cmd, "cmd", "c", "id", "Exec command(Only works on mode exp.)")
}