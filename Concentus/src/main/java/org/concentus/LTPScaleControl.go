package opus
func silk_LTP_scale_ctrl(psEnc *SilkChannelEncoder, psEncCtrl *SilkEncoderControl, condCoding int) {
    if condCoding == CODE_INDEPENDENTLY {
        round_loss := psEnc.PacketLoss_perc + psEnc.nFramesPerPacket
        psEnc.indices.LTP_scaleIndex = byte(silk_LIMIT(silk_SMULWB(silk_SMULBB(round_loss, psEncCtrl.LTPredCodGain_Q7), 51), 0, 2))
    } else {
        psEnc.indices.LTP_scaleIndex = 0
    }
    psEncCtrl.LTP_scale_Q14 = SilkTables.silk_LTPScales_table_Q14[psEnc.indices.LTP_scaleIndex]
}